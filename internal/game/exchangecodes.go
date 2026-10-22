package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Redeem codes (Settings > Redeem, view/setting/panels/settingsredeempanel.lua,
// ExchangeCodeUseCommand). The client only shows exchangecode_use_ok ("Code
// exchanged! Rewards have been sent via mail.") on success: the rewards come
// as a mail. Refusals show gametip exchange_code_error_<result>. Nothing was
// captured: the replies are built from the Lua alone.

// SC_11509 results, gametip exchange_code_error_<n>
var (
	errCodeUnknown  error = refusal{"unknown code", 20}     // "This gift code does not exist."
	errCodeExpired  error = refusal{"expired code", 6}      // "This gift code cannot be used anymore."
	errCodeRedeemed error = refusal{"code already used", 7} // "This gift code has already been used."
)

// exchangeCode is a code of server/exchange_codes.json.
type exchangeCode struct {
	Title, Sender, Content string
	Rewards                [][3]uint32
	// nil when the code always works, else when it does (server clock,
	// activity_template's ["timer", start, end] format)
	Window *gamedata.Window
}

// exchangeCodes are the redeem codes by their upper case spelling; the
// back-office rewrites the file and reloads them while the server runs
// (admin_content.go), hence the lock.
var (
	exchangeCodes   map[string]exchangeCode
	exchangeCodesMu sync.RWMutex
)

// lookupExchangeCode is the code spelled key (upper case).
func lookupExchangeCode(key string) (exchangeCode, bool) {
	exchangeCodesMu.RLock()
	defer exchangeCodesMu.RUnlock()
	code, ok := exchangeCodes[key]
	return code, ok
}

// loadExchangeCodes reads server/exchange_codes.json, the codes the server
// accepts (they are made up by whoever runs it; the client's gift_group
// table only describes the official per-player codes, which no server can
// check without the official list):
//
//	{"<CODE>": {"title": "...", "sender": "...", "content": "...",
//	            "rewards": [[type, id, count], ...],
//	            "time": ["timer", [[y,m,d],[h,m,s]], [[y,m,d],[h,m,s]]]}}
//
// Codes match without regard to case. rewards (DROPINFO type, id, count) is
// what the mail attaches; title defaults to the code, sender to
// Headquarters, time to always. Without the file no code works.
func loadExchangeCodes() error {
	raw, err := gamedata.ServerFile("exchange_codes.json")
	if err != nil {
		return err
	}
	codes, err := parseExchangeCodes(raw)
	if err != nil {
		return err
	}
	exchangeCodesMu.Lock()
	exchangeCodes = codes
	exchangeCodesMu.Unlock()
	return nil
}

// parseExchangeCodes reads exchange_codes.json's codes, none for no file.
func parseExchangeCodes(raw []byte) (map[string]exchangeCode, error) {
	codes := map[string]exchangeCode{}
	if raw == nil {
		return codes, nil
	}
	var file map[string]struct {
		Title   string          `json:"title"`
		Sender  string          `json:"sender"`
		Content string          `json:"content"`
		Rewards [][3]uint32     `json:"rewards"`
		Time    json.RawMessage `json:"time"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("exchange_codes.json: %w", err)
	}
	for key, row := range file {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" || len(row.Rewards) == 0 {
			logger.WithFields("ExchangeCode", logger.FieldValue("code", key)).Warn("exchange_codes.json: code without rewards left out")
			continue
		}
		code := exchangeCode{Title: row.Title, Sender: row.Sender, Content: row.Content, Rewards: row.Rewards}
		if code.Title == "" {
			code.Title = key
		}
		if len(row.Time) > 0 {
			var window gamedata.Window
			if err := json.Unmarshal(row.Time, &window); err != nil {
				return nil, fmt.Errorf("exchange_codes.json %s: %w", key, err)
			}
			code.Window = &window
		}
		codes[key] = code
	}
	return codes, nil
}

// mail is what redeeming code sends.
func (code exchangeCode) mail() Mail {
	mail := Mail{Title: code.Title, Sender: code.Sender, Content: code.Content}
	for _, reward := range code.Rewards {
		mail.Attachments = append(mail.Attachments, dropInfo(reward[0], reward[1], reward[2]))
	}
	return mail
}

// handleExchangeCode answers ExchangeCodeUseCommand (CS_11508): a code of
// exchangeCodes works once per commander while its window is open, and
// mails its rewards. platform is the SDK channel, ignored. A code whose
// window hasn't opened yet is unknown.
func (game *Game) handleExchangeCode(client *connection.Client, req *protobuf.CS_11508) error {
	ctx, now := context.Background(), time.Now()
	key := strings.ToUpper(strings.TrimSpace(req.GetKey()))
	err := game.tx(ctx, client, func(tx *Tx) error {
		code, ok := lookupExchangeCode(key)
		if !ok {
			return errCodeUnknown
		}
		if code.Window != nil && !code.Window.Open(now) {
			if !code.Window.End.IsZero() && !now.Before(code.Window.End) {
				return errCodeExpired
			}
			return errCodeUnknown
		}
		used, err := tx.CountExchangeCodeRedemptions(ctx, db.CountExchangeCodeRedemptionsParams{CommanderID: client.CommanderID, Code: key})
		if err != nil {
			return err
		}
		if used > 0 {
			return errCodeRedeemed
		}
		mail := code.mail()
		if _, err := mail.check(); err != nil {
			logger.WithFields("ExchangeCode", logger.FieldValue("code", key)).Warn(fmt.Sprintf("exchange_codes.json: %v", err))
			return errCodeUnknown
		}
		if err := tx.AddExchangeCodeRedemption(ctx, db.AddExchangeCodeRedemptionParams{CommanderID: client.CommanderID, Code: key, RedeemedAt: now.Unix()}); err != nil {
			return err
		}
		return tx.SendMail(mail)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_11509{Result: result(err)})
}
