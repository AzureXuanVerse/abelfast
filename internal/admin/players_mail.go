package admin

import (
	"net/http"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

// mailCountLimit is MAIL_COUNT_LIMIT (const.lua): the inbox the client
// counts against, past which it warns.
const mailCountLimit = 100

type playerAttachment struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Count  uint32 `json:"count"`
	Name   string `json:"name,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

type playerMail struct {
	ID          uint32             `json:"id"`
	Date        int64              `json:"date"`
	Expires     int64              `json:"expires,omitempty"`
	Title       string             `json:"title"`
	Sender      string             `json:"sender"`
	Content     string             `json:"content"`
	Attachments []playerAttachment `json:"attachments"`
	Read        bool               `json:"read"`
	Claimed     bool               `json:"claimed"`
	Important   bool               `json:"important"`
}

type playerMails struct {
	Inbox         []playerMail `json:"inbox"`
	Storage       []playerMail `json:"storage"`
	Compensations []playerMail `json:"compensations"`
	InboxMax      uint32       `json:"inbox_max"`
	StorageMax    uint32       `json:"storage_max"`
}

func (server *Server) playerAttachments(drops db.Drops, tables *playerTables) []playerAttachment {
	list := make([]playerAttachment, 0, len(drops))
	for _, d := range drops {
		row, ok := server.names[d[0]][d[1]]
		if !ok {
			row = tables.names[d[0]][d[1]]
		}
		if d[0] == dropSkinTimed {
			row = server.names[dropSkin][d[1]]
		}
		list = append(list, playerAttachment{Type: d[0], ID: d[1], Count: d[2], Name: row.Name, Rarity: row.Rarity, Icon: row.Icon})
	}
	return list
}

// handlePlayerMails lists a commander's inbox, storage and compensations,
// newest first.
func (server *Server) handlePlayerMails(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	mails := playerMails{Inbox: []playerMail{}, Storage: []playerMail{}, Compensations: []playerMail{}, InboxMax: mailCountLimit, StorageMax: game.MaxStoredMails(commander.Level)}
	inbox, err := server.Store.ListInboxMails(ctx, id)
	if err != nil {
		return nil, err
	}
	storage, err := server.Store.ListArchivedMailsPage(ctx, db.ListArchivedMailsPageParams{CommanderID: id, Count: 10000})
	if err != nil {
		return nil, err
	}
	for _, list := range []struct {
		rows []db.Mail
		to   *[]playerMail
	}{{inbox, &mails.Inbox}, {storage, &mails.Storage}} {
		for i := len(list.rows) - 1; i >= 0; i-- {
			mail := list.rows[i]
			*list.to = append(*list.to, playerMail{
				ID: mail.ID, Date: mail.Date, Title: mail.Title, Sender: mail.Sender, Content: mail.Content,
				Attachments: server.playerAttachments(mail.Attachments, tables),
				Read:        mail.IsRead, Claimed: mail.IsClaimed, Important: mail.IsImportant,
			})
		}
	}
	compensations, err := server.Store.AdminListAllCompensations(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	for _, row := range compensations {
		if row.Expires <= now {
			continue
		}
		mails.Compensations = append(mails.Compensations, playerMail{
			ID: row.ID, Date: row.Date, Expires: row.Expires, Title: row.Title, Sender: row.Sender, Content: row.Content,
			Attachments: server.playerAttachments(row.Attachments, tables), Claimed: row.IsClaimed,
		})
	}
	return mails, nil
}

// deleteMailRow deletes a commander's mail or compensation, kicking it
// first if it plays: its client keeps the mails it loaded.
func (server *Server) deleteMailRow(r *http.Request, what string, remove func(*db.Queries, uint32, uint32) (int64, error)) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	item, err := pathUint(r, "item")
	if err != nil {
		return nil, err
	}
	return kicked(server.Game.EditCommanderRows(r.Context(), commander.ID, func(queries *db.Queries) error {
		deleted, err := remove(queries, commander.ID, item)
		if err == nil && deleted == 0 {
			return notFound("no " + what + " " + itoa(item))
		}
		return err
	}))
}

// handlePlayerDeleteMail deletes a mail, attachments and all.
func (server *Server) handlePlayerDeleteMail(r *http.Request) (any, error) {
	return server.deleteMailRow(r, "mail", func(queries *db.Queries, id, mail uint32) (int64, error) {
		return queries.AdminDeleteMail(r.Context(), db.AdminDeleteMailParams{CommanderID: id, ID: mail})
	})
}

// handlePlayerDeleteCompensation deletes a compensation.
func (server *Server) handlePlayerDeleteCompensation(r *http.Request) (any, error) {
	return server.deleteMailRow(r, "compensation", func(queries *db.Queries, id, compensation uint32) (int64, error) {
		return queries.AdminDeleteCompensation(r.Context(), db.AdminDeleteCompensationParams{CommanderID: id, ID: compensation})
	})
}
