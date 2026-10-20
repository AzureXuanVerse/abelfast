package game

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The secondary password (Settings, Secondary Password; SecondaryPWDProxy,
// SecondaryPWDMgr, SecondaryPasswordLayer / Mediator). None of its packets
// were captured: everything is built from the Lua and the
// secondary_password_help text.

// SC_11604.state (SecondaryPWDProxy)
const (
	secondaryPasswordNone     = 0 // no password, nothing is protected
	secondaryPasswordLocked   = 1 // set, not entered since this login
	secondaryPasswordUnlocked = 2 // entered this login: not asked again
)

const (
	// secondaryPasswordMaxFails is how many wrong passwords in a row lock
	// it (SecondaryPasswordMediator refetches the state at fail_count 5,
	// secondary_password_help: "5 times in a row").
	secondaryPasswordMaxFails = 5
	// secondaryPasswordCooldown is how long it stays locked then. A guess:
	// neither the Lua, the tables nor the wiki say (the help text only
	// says "for some time").
	secondaryPasswordCooldown = time.Hour
	// secondaryPasswordLength: SecondaryPasswordLayer takes 6 digits
	secondaryPasswordLength = 6
	// secondaryPasswordNoticeMax bounds the hint, which the client clips
	// to 20 characters (SecondaryPasswordMediator.ClipUnicodeStr); a
	// little slack for how it counts.
	secondaryPasswordNoticeMax = 32
	// secondaryPasswordMaxSystem is the highest SecondaryPWDMgr operation
	// (UNLOCK_SHIP 1, UNLOCK_COMMANDER 2, RESOLVE_EQUIPMENT 3,
	// CREATE_INHERIT 4) a password can protect.
	secondaryPasswordMaxSystem = 4
	pbkdf2Iterations           = 100_000
)

var (
	// "Your Secondary Password is incorrect": the client counts the
	// failure (secondarypassword_incorrectpwd_error) and refetches the
	// state at the 5th
	errWrongPassword error = refusal{"wrong secondary password", 9}
	// still locked out: the client refetches the state and shows the
	// time left (tips_fail_secondarypwd_much_times)
	errPasswordLockedOut error = refusal{"secondary password locked out", 40}
)

// secondaryUnlocks are the commanders who entered their secondary password
// since they logged in (state 2): "you won't need to enter it again until
// the next time you log in". A login (CS_11001 timestamp 0, not a
// reconnect's 1) or a server restart forgets it.
type secondaryUnlocks struct {
	mu         sync.Mutex
	commanders map[uint32]bool
}

func (game *Game) setSecondaryUnlocked(commanderID uint32, unlocked bool) {
	unlocks := &game.secondaryUnlocks
	unlocks.mu.Lock()
	defer unlocks.mu.Unlock()
	if unlocks.commanders == nil {
		unlocks.commanders = map[uint32]bool{}
	}
	if unlocked {
		unlocks.commanders[commanderID] = true
	} else {
		delete(unlocks.commanders, commanderID)
	}
}

func (game *Game) secondaryUnlocked(commanderID uint32) bool {
	unlocks := &game.secondaryUnlocks
	unlocks.mu.Lock()
	defer unlocks.mu.Unlock()
	return unlocks.commanders[commanderID]
}

// hashSecondaryPassword is "<salt hex>$<pbkdf2-sha256 hex>".
func hashSecondaryPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, sha256.Size)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + "$" + hex.EncodeToString(key), nil
}

func checkSecondaryPassword(hash, password string) bool {
	saltHex, keyHex, ok := strings.Cut(hash, "$")
	if !ok {
		return false
	}
	salt, err1 := hex.DecodeString(saltHex)
	want, err2 := hex.DecodeString(keyHex)
	if err1 != nil || err2 != nil {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, len(want))
	return err == nil && subtle.ConstantTimeCompare(key, want) == 1
}

// validSecondaryPassword is what SecondaryPasswordLayer lets through: 6
// digits.
func validSecondaryPassword(password string) bool {
	if len(password) != secondaryPasswordLength {
		return false
	}
	for _, c := range []byte(password) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// validSecondarySystems checks a system_list: SecondaryPWDMgr operations,
// each once.
func validSecondarySystems(systems []uint32) bool {
	seen := map[uint32]bool{}
	for _, system := range systems {
		if system == 0 || system > secondaryPasswordMaxSystem || seen[system] {
			return false
		}
		seen[system] = true
	}
	return true
}

// secondaryPassword is the commander's password row, ok false when it has
// none. A lockout that ran out is cleared (fail_count back to 0).
func (game *Game) secondaryPassword(ctx context.Context, commanderID uint32, now time.Time) (db.CommanderSecondaryPassword, bool, error) {
	row, err := game.DB.GetSecondaryPassword(ctx, commanderID)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	if row.FailCd != 0 && now.Unix() >= row.FailCd {
		row.FailCount, row.FailCd = 0, 0
		if err := game.DB.SetSecondaryPasswordFails(ctx, db.SetSecondaryPasswordFailsParams{CommanderID: commanderID}); err != nil {
			return row, false, err
		}
	}
	return row, true, nil
}

// enterSecondaryPassword checks a password the commander typed (CS_11607,
// CS_11609): refused while locked out (40), a wrong one counts a failure
// (9) and locks it for secondaryPasswordCooldown at the 5th; a right one
// clears the failures and unlocks it until the next login.
func (game *Game) enterSecondaryPassword(ctx context.Context, commanderID uint32, row db.CommanderSecondaryPassword, password string, now time.Time) error {
	if row.FailCd != 0 {
		return errPasswordLockedOut
	}
	if !checkSecondaryPassword(row.PasswordHash, password) {
		fails := db.SetSecondaryPasswordFailsParams{CommanderID: commanderID, FailCount: row.FailCount + 1}
		if fails.FailCount >= secondaryPasswordMaxFails {
			fails.FailCd = now.Add(secondaryPasswordCooldown).Unix()
		}
		if err := game.DB.SetSecondaryPasswordFails(ctx, fails); err != nil {
			return err
		}
		return errWrongPassword
	}
	if row.FailCount != 0 {
		if err := game.DB.SetSecondaryPasswordFails(ctx, db.SetSecondaryPasswordFailsParams{CommanderID: commanderID}); err != nil {
			return err
		}
	}
	game.setSecondaryUnlocked(commanderID, true)
	return nil
}

// handleSecondaryPassword answers FetchSecondaryPasswordCommand (after the
// login burst, after a reconnect, and when a wrong password needs the
// lockout): SecondaryPWDProxy.SetData. state 0 without a password, else 1
// or 2 (entered since this login); fail_cd is the end of a lockout (unix
// seconds, GetPermissionState), notice the hint the input screen shows,
// system_list the protected operations.
func (game *Game) handleSecondaryPassword(client *connection.Client, req *protobuf.CS_11603) error {
	row, ok, err := game.secondaryPassword(context.Background(), client.CommanderID, time.Now())
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_11604{
			State:     proto.Uint32(secondaryPasswordNone),
			FailCount: proto.Uint32(0),
			FailCd:    proto.Uint32(0),
			Notice:    proto.String(""),
		})
	}
	state := uint32(secondaryPasswordLocked)
	if game.secondaryUnlocked(client.CommanderID) {
		state = secondaryPasswordUnlocked
	}
	return client.Send(&protobuf.SC_11604{
		State:      proto.Uint32(state),
		SystemList: row.SystemList,
		FailCount:  proto.Uint32(row.FailCount),
		FailCd:     proto.Uint32(uint32(row.FailCd)),
		Notice:     proto.String(row.Notice),
	})
}

// handleSetSecondaryPassword answers SetSecondaryPasswordCommand (Settings,
// turning the secondary password on): the password, its hint and the
// operations it protects (SecondaryPWDMgr.LIMITED_OPERATION). The client
// then has it in state 1 (OnFirstSet). Refused when one is already set; a
// refusal shows errorTip and refetches the state.
func (game *Game) handleSetSecondaryPassword(client *connection.Client, req *protobuf.CS_11605) error {
	ctx := context.Background()
	systems := req.GetSystemList()
	if !validSecondaryPassword(req.GetPassword()) || !validSecondarySystems(systems) || len(systems) == 0 ||
		!utf8.ValidString(req.GetNotice()) || utf8.RuneCountInString(req.GetNotice()) > secondaryPasswordNoticeMax {
		return client.Send(&protobuf.SC_11606{Result: result(errRefused)})
	}
	if _, ok, err := game.secondaryPassword(ctx, client.CommanderID, time.Now()); err != nil {
		return err
	} else if ok {
		return client.Send(&protobuf.SC_11606{Result: result(errRefused)})
	}
	hash, err := hashSecondaryPassword(req.GetPassword())
	if err != nil {
		return err
	}
	err = game.DB.CreateSecondaryPassword(ctx, db.CreateSecondaryPasswordParams{
		CommanderID:  client.CommanderID,
		PasswordHash: hash,
		Notice:       req.GetNotice(),
		SystemList:   systems,
	})
	if err != nil {
		return err
	}
	game.setSecondaryUnlocked(client.CommanderID, false)
	return client.Send(&protobuf.SC_11606{Result: result(nil)})
}

// handleSecondaryPasswordSettings answers SetSecondaryPasswordSettingsCommand
// (Settings, changing which operations are protected, or turning it off
// with none), behind the password: an empty system_list removes the
// password (state 0), any other replaces the list and the password counts
// as entered (state 2, OnSettingsChange).
func (game *Game) handleSecondaryPasswordSettings(client *connection.Client, req *protobuf.CS_11607) error {
	ctx := context.Background()
	now := time.Now()
	systems := req.GetSystemList()
	if !validSecondarySystems(systems) {
		return client.Send(&protobuf.SC_11608{Result: result(errRefused)})
	}
	row, ok, err := game.secondaryPassword(ctx, client.CommanderID, now)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_11608{Result: result(errRefused)})
	}
	if err := game.enterSecondaryPassword(ctx, client.CommanderID, row, req.GetPassword(), now); err != nil {
		if refused(err) {
			return client.Send(&protobuf.SC_11608{Result: result(err)})
		}
		return err
	}
	if len(systems) == 0 {
		err = game.DB.DeleteSecondaryPassword(ctx, client.CommanderID)
		game.setSecondaryUnlocked(client.CommanderID, false)
	} else {
		err = game.DB.SetSecondaryPasswordSystems(ctx, db.SetSecondaryPasswordSystemsParams{CommanderID: client.CommanderID, SystemList: systems})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11608{Result: result(nil)})
}

// handleConfirmSecondaryPassword answers ConfirmSecondaryPasswordCommand,
// the password asked before a protected operation (unlocking a ship or a
// meowfficer, scrapping important gear): the right one unlocks it until
// the next login (state 2) and the operation goes on.
func (game *Game) handleConfirmSecondaryPassword(client *connection.Client, req *protobuf.CS_11609) error {
	ctx := context.Background()
	now := time.Now()
	row, ok, err := game.secondaryPassword(ctx, client.CommanderID, now)
	if err != nil {
		return err
	}
	if !ok {
		return client.Send(&protobuf.SC_11610{Result: result(errRefused)})
	}
	if err := game.enterSecondaryPassword(ctx, client.CommanderID, row, req.GetPassword(), now); err != nil {
		if refused(err) {
			return client.Send(&protobuf.SC_11610{Result: result(err)})
		}
		return err
	}
	return client.Send(&protobuf.SC_11610{Result: result(nil)})
}
