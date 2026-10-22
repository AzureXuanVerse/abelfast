package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gateway"
)

// The inner login accounts (inner_accounts: the client's own username /
// password panels) and the commander each one has.

type accountCommander struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Level  uint32 `json:"level"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
	Online bool   `json:"online"`
}

type accountRow struct {
	Username     string            `json:"username"`
	MailBox      string            `json:"mail_box"`
	AccountID    uint32            `json:"account_id"`
	RegisteredAt int64             `json:"registered_at"`
	Commander    *accountCommander `json:"commander"`
	// the commander's secondary password: "" none, "set", or "locked" (too
	// many wrong ones, until fail_cd)
	Secondary string `json:"secondary"`
}

func secondaryState(set bool, failCD int64) string {
	switch {
	case !set:
		return ""
	case failCD > time.Now().Unix():
		return "locked"
	}
	return "set"
}

// handleAccounts lists the inner login accounts with their commander.
func (server *Server) handleAccounts(r *http.Request) (any, error) {
	rows, err := server.Store.AdminListInnerAccounts(r.Context())
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(r.Context())
	if err != nil {
		return nil, err
	}
	accounts := make([]accountRow, 0, len(rows))
	for _, row := range rows {
		account := accountRow{Username: row.Username, MailBox: row.MailBox, AccountID: row.AccountID, RegisteredAt: row.RegisteredAt}
		if row.CommanderID != 0 {
			id := uint32(row.CommanderID)
			account.Commander = &accountCommander{
				ID: id, Name: row.CommanderName, Level: uint32(row.CommanderLevel),
				Icon: icons[id].Icon, Rarity: icons[id].Rarity, Online: server.Game.IsOnline(id),
			}
			account.Secondary = secondaryState(row.SecondarySet, row.SecondaryFailCd)
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

type secondaryDetail struct {
	State     string   `json:"state"`
	FailCount uint32   `json:"fail_count"`
	FailCD    int64    `json:"fail_cd"`
	Systems   []uint32 `json:"systems"`
}

type redemption struct {
	Code string `json:"code"`
	At   int64  `json:"at"`
}

type accountDetail struct {
	accountRow
	SecondaryPassword *secondaryDetail `json:"secondary_password"`
	Codes             []redemption     `json:"codes"`
}

// account is the {id} inner account (by account id), 404 when there is none.
func (server *Server) account(r *http.Request) (db.InnerAccount, error) {
	id, err := pathID(r)
	if err != nil {
		return db.InnerAccount{}, err
	}
	account, err := server.Store.AdminGetInnerAccount(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return account, notFound("no inner account " + itoa(id))
	}
	return account, err
}

// accountCommanderID is the commander of an account, 0 when it has none.
func (server *Server) accountCommanderID(r *http.Request, account uint32) (uint32, error) {
	commander, err := server.Store.AdminCommanderByAccount(r.Context(), account)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return commander.ID, err
}

// handleAccount is an inner account with its commander's secondary password
// and the redeem codes it used.
func (server *Server) handleAccount(r *http.Request) (any, error) {
	account, err := server.account(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	detail := accountDetail{
		accountRow: accountRow{Username: account.Username, MailBox: account.MailBox, AccountID: account.AccountID, RegisteredAt: account.RegisteredAt},
		Codes:      []redemption{},
	}
	commander, err := server.Store.AdminCommanderByAccount(ctx, account.AccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return detail, nil
	}
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	detail.Commander = &accountCommander{ID: commander.ID, Name: commander.Name, Level: commander.Level, Icon: icons[commander.ID].Icon, Rarity: icons[commander.ID].Rarity, Online: server.Game.IsOnline(commander.ID)}
	secondary, err := server.Store.GetSecondaryPassword(ctx, commander.ID)
	switch {
	case err == nil:
		detail.Secondary = secondaryState(true, secondary.FailCd)
		detail.SecondaryPassword = &secondaryDetail{State: detail.Secondary, FailCount: secondary.FailCount, FailCD: secondary.FailCd, Systems: []uint32(secondary.SystemList)}
		if detail.SecondaryPassword.Systems == nil {
			detail.SecondaryPassword.Systems = []uint32{}
		}
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}
	codes, err := server.Store.AdminListRedemptions(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	for _, code := range codes {
		detail.Codes = append(detail.Codes, redemption{Code: code.Code, At: code.RedeemedAt})
	}
	return detail, nil
}

// maxPassword and maxMailBox bound what the back-office stores.
const (
	maxPassword = 128
	maxMailBox  = 254
)

// handleAccountPassword sets an inner account's password, hashed as the
// inner login checks it.
func (server *Server) handleAccountPassword(r *http.Request) (any, error) {
	account, err := server.account(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if len(body.Password) > maxPassword {
		return nil, badRequest("password: too long")
	}
	err = server.Store.AdminSetInnerPassword(r.Context(), db.AdminSetInnerPasswordParams{PasswordHash: gateway.PasswordHash(body.Password), AccountID: account.AccountID})
	return struct{}{}, err
}

// handleAccountMailBox sets an inner account's mail box (CS_10001 mail_box,
// which the register panel leaves empty).
func (server *Server) handleAccountMailBox(r *http.Request) (any, error) {
	account, err := server.account(r)
	if err != nil {
		return nil, err
	}
	var body struct {
		MailBox string `json:"mail_box"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	if len(body.MailBox) > maxMailBox {
		return nil, badRequest("mail_box: too long")
	}
	err = server.Store.AdminSetInnerMailBox(r.Context(), db.AdminSetInnerMailBoxParams{MailBox: body.MailBox, AccountID: account.AccountID})
	return struct{}{}, err
}

// commanderOf is the {id} account's commander, a 400 when it has none.
func (server *Server) commanderOf(r *http.Request) (uint32, error) {
	account, err := server.account(r)
	if err != nil {
		return 0, err
	}
	id, err := server.accountCommanderID(r, account.AccountID)
	if err == nil && id == 0 {
		return 0, badRequest("the account has no commander")
	}
	return id, err
}

// handleAccountUnlink detaches the account's commander (see
// game.UnlinkCommander).
func (server *Server) handleAccountUnlink(r *http.Request) (any, error) {
	id, err := server.commanderOf(r)
	if err != nil {
		return nil, err
	}
	was, err := server.Game.UnlinkCommander(r.Context(), id)
	if errors.Is(err, game.ErrUnlinkedTaken) {
		return nil, httpError{http.StatusConflict, err.Error()}
	}
	return kicked(was, err)
}

// handleAccountClearSecondary removes the commander's secondary password.
func (server *Server) handleAccountClearSecondary(r *http.Request) (any, error) {
	id, err := server.commanderOf(r)
	if err != nil {
		return nil, err
	}
	return kicked(server.Game.ResetSecondaryPassword(r.Context(), id))
}

// handleAccountForgetCode forgets that the commander used a redeem code, so
// it can redeem it again (CS_11508 checks exchange_code_redemptions).
func (server *Server) handleAccountForgetCode(r *http.Request) (any, error) {
	id, err := server.commanderOf(r)
	if err != nil {
		return nil, err
	}
	removed, err := server.Store.AdminDeleteRedemption(r.Context(), db.AdminDeleteRedemptionParams{CommanderID: id, Code: r.PathValue("code")})
	if err != nil {
		return nil, err
	}
	if removed == 0 {
		return nil, notFound("code " + r.PathValue("code") + " not used")
	}
	return struct{}{}, nil
}

// handleAccountDelete deletes an inner account, its login only: its
// commander stays under the account id, which registering the same username
// again gives back. A commander playing is kicked first.
func (server *Server) handleAccountDelete(r *http.Request) (any, error) {
	account, err := server.account(r)
	if err != nil {
		return nil, err
	}
	id, err := server.accountCommanderID(r, account.AccountID)
	if err != nil {
		return nil, err
	}
	if id != 0 {
		if _, err := server.Game.Kick(id); err != nil {
			return nil, err
		}
	}
	return struct{}{}, server.Store.AdminDeleteInnerAccount(r.Context(), account.AccountID)
}
