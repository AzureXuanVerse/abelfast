package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

const (
	ticketKeySetting = "ticket_key"
	ticketPayloadLen = 8  // account id + issue time, both big endian uint32
	ticketMACLen     = 16 // truncated HMAC-SHA256
)

// LoadTicketKey returns the key server tickets are signed with, generating it
// on first run. It lives in the database so tickets (and thus the client's
// reconnects) survive restarts.
func LoadTicketKey(ctx context.Context, queries *db.Queries) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := queries.InsertSettingIfMissing(ctx, db.InsertSettingIfMissingParams{Key: ticketKeySetting, Value: key}); err != nil {
		return nil, err
	}
	return queries.GetSetting(ctx, ticketKeySetting)
}

// issueTicket returns a server ticket for account: hex(account, issued at, MAC).
func (gateway *Gateway) issueTicket(account uint32) string {
	ticket := make([]byte, ticketPayloadLen, ticketPayloadLen+ticketMACLen)
	binary.BigEndian.PutUint32(ticket[0:4], account)
	binary.BigEndian.PutUint32(ticket[4:8], uint32(time.Now().Unix()))
	return hex.EncodeToString(append(ticket, gateway.ticketMAC(ticket)...))
}

// account returns the account a ticket was issued to, if it is one of ours.
func (gateway *Gateway) account(ticket string) (uint32, bool) {
	raw, err := hex.DecodeString(ticket)
	if err != nil || len(raw) != ticketPayloadLen+ticketMACLen {
		return 0, false
	}
	payload, mac := raw[:ticketPayloadLen], raw[ticketPayloadLen:]
	if !hmac.Equal(mac, gateway.ticketMAC(payload)) {
		return 0, false
	}
	return binary.BigEndian.Uint32(payload[0:4]), true
}

func (gateway *Gateway) ticketMAC(payload []byte) []byte {
	mac := hmac.New(sha256.New, gateway.TicketKey)
	mac.Write(payload)
	return mac.Sum(nil)[:ticketMACLen]
}
