package game

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	// maxFriends and maxBlacklist are the client's MAX_FRIEND_COUNT and
	// MAX_BLACKLIST_COUNT (const.lua), no config table has them.
	maxFriends   = 50
	maxBlacklist = 50
	// recommendedPlayers is how many players CS_50014 suggests (the search
	// page's refresh, every 10 s at most); the client lists any number.
	recommendedPlayers = 10
	// searchByID is CS_50001.type for a numeric keyword
	// (SearchFriendCommand), anything else searches names.
	searchByID = 0
	// rejectAll is CS_50009.id for "reject all" (FriendMediator).
	rejectAll = 0
)

// The friend replies' result codes the Lua tells apart, error_message rows
// it shows after its own tip (errorTip). What the client never lets through
// (befriending or blocking oneself or a friend...) gets errRefused.
var (
	// "Already Reached Limit.": a friend list or the blacklist is full;
	// on 50007 the client adds friend_max_count_1 (the other side's list)
	errLimitReached error = refusal{"limit reached", 6}
	// add_friend_fail_tip9 on 50004 (result 1 would read "request sent")
	errRequestFailed error = refusal{"request failed", 9}
	// "Player Does Not Exist.", friend_searchFriend_noPlayer on 50002
	errNoPlayer error = refusal{"no such player", 21}
	// "Friend Does Not Exist.", friend_offline on 50106
	errNotFriend error = refusal{"not a friend or offline", 28}
)

// displayInfo is the commander's portrait as other players see it: its
// flagship (the first secretary), as captured: icon the ship's template,
// skin its skin, marry_flag its oath time (PlayerAttire.proposeTime).
// transform_flag stays 0, retrofits aren't kept. Without a secretary, or
// when it can't be read, the starter ship (a new account's capture).
func (game *Game) displayInfo(commander db.Commander) *protobuf.DISPLAYINFO {
	flagship, err := game.DB.GetFlagshipDisplay(context.Background(), commander.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.WithFields("Friend", logger.FieldValue("commander", commander.ID)).Warn("flagship unreadable: " + err.Error())
		}
		return &protobuf.DISPLAYINFO{Icon: proto.Uint32(commander.StarterShip)}
	}
	return &protobuf.DISPLAYINFO{
		Icon:      proto.Uint32(flagship.TemplateID),
		Skin:      proto.Uint32(flagship.SkinID),
		MarryFlag: proto.Uint32(flagship.Propose),
	}
}

// playerInfoP50 is a commander as a world chat sender, a friend request or
// message sender, a blacklisted or suggested player (Player.New).
func (game *Game) playerInfoP50(commander db.Commander) *protobuf.PLAYER_INFO_P50 {
	return &protobuf.PLAYER_INFO_P50{
		Id:      proto.Uint32(commander.ID),
		Name:    proto.String(commander.Name),
		Lv:      proto.Uint32(commander.Level),
		Display: game.displayInfo(commander),
	}
}

// friendInfo is a commander in a friend list (Friend.New): online or not,
// and when it last logged in or out.
func (game *Game) friendInfo(commander db.Commander) *protobuf.FRIEND_INFO {
	var online uint32
	if game.isOnline(commander.ID) {
		online = 1
	}
	return &protobuf.FRIEND_INFO{
		Id:            proto.Uint32(commander.ID),
		Name:          proto.String(commander.Name),
		Lv:            proto.Uint32(commander.Level),
		Adv:           proto.String(commander.Manifesto),
		Online:        proto.Uint32(online),
		PreOnlineTime: proto.Uint32(uint32(commander.LastOnline)),
		Display:       game.displayInfo(commander),
	}
}

// detailInfo is a commander's profile card (resumeLayer, the search page):
// battle and exercise counts aren't kept, they read 0.
func (game *Game) detailInfo(ctx context.Context, commander db.Commander) (*protobuf.DETAIL_INFO, error) {
	ships, err := game.DB.CountShips(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	groups, err := game.DB.CountShipGroups(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	medals, err := displayMedals(ctx, game.DB.Queries, commander.ID)
	if err != nil {
		return nil, err
	}
	friend := game.friendInfo(commander)
	return &protobuf.DETAIL_INFO{
		MedalId:         medals,
		Id:              friend.Id,
		Name:            friend.Name,
		Title:           proto.Uint32(commander.Rank),
		Lv:              friend.Lv,
		ShipCount:       proto.Uint32(uint32(ships)),
		CollectionCount: proto.Uint32(uint32(groups)),
		Adv:             friend.Adv,
		Online:          friend.Online,
		PreOnlineTime:   friend.PreOnlineTime,
		Display:         friend.Display,
	}, nil
}

// friendList is the login's SC_50000: the friends (FriendProxy) and the
// requests waiting (NotificationProxy, which hides the blacklisted senders'
// itself).
func (game *Game) friendList(ctx context.Context, queries *db.Queries, commanderID uint32) (*protobuf.SC_50000, error) {
	friends, err := queries.ListFriends(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	requests, err := queries.ListFriendRequests(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	list := &protobuf.SC_50000{}
	for _, friend := range friends {
		list.FriendList = append(list.FriendList, game.friendInfo(friend.Commander))
	}
	for _, request := range requests {
		list.RequestList = append(list.RequestList, &protobuf.MSG_INFO_P50{
			Timestamp: proto.Uint32(uint32(request.SentAt)),
			Player:    game.playerInfoP50(request.Commander),
			Content:   proto.String(request.Content),
		})
	}
	return list, nil
}

// commanderByID is another player by id, errNoPlayer when there's none.
func commanderByID(ctx context.Context, queries *db.Queries, id uint32) (db.Commander, error) {
	commander, err := queries.GetCommander(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return commander, errNoPlayer
	}
	return commander, err
}

// handleSearchPlayer answers SearchFriendCommand's search and profile
// (resume) lookups (CS_50001): type 0 for a numeric keyword, a commander id,
// else a name. Not found is any non-zero result (friend_searchFriend_noPlayer).
// Built from the Lua alone.
func (game *Game) handleSearchPlayer(client *connection.Client, req *protobuf.CS_50001) error {
	ctx := context.Background()
	keyword := strings.TrimSpace(req.GetKeyword())
	commander, err := db.Commander{}, error(errNoPlayer)
	if req.GetType() == searchByID {
		if id, parseErr := strconv.ParseUint(keyword, 10, 32); parseErr == nil {
			commander, err = commanderByID(ctx, game.DB.Queries, uint32(id))
		}
	}
	if errors.Is(err, errNoPlayer) {
		commander, err = game.DB.GetCommanderByName(ctx, keyword)
		if errors.Is(err, sql.ErrNoRows) {
			err = errNoPlayer
		}
	}
	if refused(err) {
		return client.Send(&protobuf.SC_50002{Result: result(err)})
	}
	if err != nil {
		return err
	}
	detail, err := game.detailInfo(ctx, commander)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_50002{Result: result(nil), Player: detail})
}

// handleRecommendPlayers answers SearchFriendCommand's suggestion list
// (CS_50014, type 0), sent when the search page opens and on refresh: other
// commanders, not friends or blacklisted, most recently online first. Built
// from the Lua alone.
func (game *Game) handleRecommendPlayers(client *connection.Client, req *protobuf.CS_50014) error {
	commanders, err := game.DB.ListRecommendedCommanders(context.Background(), db.ListRecommendedCommandersParams{
		CommanderID: client.CommanderID,
		Count:       recommendedPlayers,
	})
	if err != nil {
		return err
	}
	reply := &protobuf.SC_50015{}
	for _, commander := range commanders {
		reply.PlayerList = append(reply.PlayerList, game.playerInfoP50(commander))
	}
	return client.Send(reply)
}

// handleBatchGetPlayers answers BatchGetFriendsCommand (CS_50018), the island
// looking up the players it shows: their FRIEND_INFO, unknown ids left out.
// Built from the Lua alone.
func (game *Game) handleBatchGetPlayers(client *connection.Client, req *protobuf.CS_50018) error {
	reply := &protobuf.SC_50019{}
	if ids := req.GetUserIdList(); len(ids) > 0 {
		commanders, err := game.DB.ListCommandersByID(context.Background(), ids)
		if err != nil {
			return err
		}
		for _, commander := range commanders {
			reply.UserList = append(reply.UserList, game.friendInfo(commander))
		}
	}
	return client.Send(reply)
}

// handleThemeAuthor answers GetThemeTemplatePlayerInfoCommand (CS_50113):
// the author of a shared backyard theme (shop or collection), shown on the
// theme's card as CourtYardThemeOwner. Built from the Lua alone.
func (game *Game) handleThemeAuthor(client *connection.Client, req *protobuf.CS_50113) error {
	commander, err := commanderByID(context.Background(), game.DB.Queries, req.GetUserId())
	if refused(err) {
		return client.Send(&protobuf.SC_50114{Result: result(err)})
	}
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_50114{Result: result(nil), Player: game.playerInfoP50(commander)})
}

// handleSendFriendRequest answers SendFriendRequestCommand (CS_50003): the
// request waits in the target's notifications with its note, pushed to it
// right away when it is online (SC_50005), else sent at its next login
// (SC_50000). Sending again replaces the note. The client already refuses
// itself, a friend and a full friend list; a blocked sender's request is
// kept, the target's client hides it. Built from the Lua alone.
func (game *Game) handleSendFriendRequest(client *connection.Client, req *protobuf.CS_50003) error {
	ctx := context.Background()
	now := time.Now()
	var sender db.Commander
	err := game.tx(ctx, client, func(tx *Tx) error {
		if req.GetId() == client.CommanderID {
			return errRequestFailed
		}
		if _, err := commanderByID(ctx, tx.Queries, req.GetId()); err != nil {
			return err
		}
		friend, err := tx.CountFriend(ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: req.GetId()})
		if err != nil {
			return err
		}
		friends, err := tx.CountFriends(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if friend > 0 || friends >= maxFriends {
			return errRequestFailed
		}
		if sender, err = tx.GetCommander(ctx, client.CommanderID); err != nil {
			return err
		}
		return tx.SetFriendRequest(ctx, db.SetFriendRequestParams{
			CommanderID: req.GetId(),
			SenderID:    client.CommanderID,
			Content:     req.GetContent(),
			SentAt:      now.Unix(),
		})
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		// NotificationProxy keeps the first request of a sender it has
		game.deliver(client, req.GetId(), &protobuf.SC_50005{Msg: &protobuf.MSG_INFO_P50{
			Timestamp: proto.Uint32(uint32(now.Unix())),
			Player:    game.playerInfoP50(sender),
			Content:   proto.String(req.GetContent()),
		}})
	}
	return client.Send(&protobuf.SC_50004{Result: result(err)})
}

// handleAcceptFriendRequest answers AcceptFriendRequestCommand (CS_50006):
// both become friends, the request (and one the other way) is gone and the
// sender leaves the blacklist (the client asked first, and drops it from
// its list on result 0). The client only drops the request: both sides learn
// of the new friend from SC_50008, the sender when it is online, else at its
// next login. Result 6 when the sender's friend list is full. Built from the
// Lua alone.
func (game *Game) handleAcceptFriendRequest(client *connection.Client, req *protobuf.CS_50006) error {
	ctx := context.Background()
	now := time.Now().Unix()
	var self, sender db.Commander
	err := game.tx(ctx, client, func(tx *Tx) error {
		var err error
		if sender, err = commanderByID(ctx, tx.Queries, req.GetId()); err != nil {
			return err
		}
		removed, err := tx.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: client.CommanderID, SenderID: sender.ID})
		if err != nil {
			return err
		}
		if removed == 0 {
			return errRefused
		}
		if self, err = tx.GetCommander(ctx, client.CommanderID); err != nil {
			return err
		}
		friend, err := tx.CountFriend(ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: sender.ID})
		if err != nil || friend > 0 {
			return err // already friends: the request is just gone
		}
		for _, side := range [][2]uint32{{client.CommanderID, sender.ID}, {sender.ID, client.CommanderID}} {
			friends, err := tx.CountFriends(ctx, side[0])
			if err != nil {
				return err
			}
			if friends >= maxFriends {
				if side[0] == client.CommanderID {
					return errRefused // the client checks its own list
				}
				return errLimitReached
			}
			if err := tx.AddFriend(ctx, db.AddFriendParams{CommanderID: side[0], FriendID: side[1], Since: now}); err != nil {
				return err
			}
		}
		if _, err := tx.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: sender.ID, SenderID: client.CommanderID}); err != nil {
			return err
		}
		_, err = tx.DeleteBlacklist(ctx, db.DeleteBlacklistParams{CommanderID: client.CommanderID, BlockedID: sender.ID})
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err == nil {
		game.deliver(client, client.CommanderID, &protobuf.SC_50008{Player: game.friendInfo(sender)})
		game.deliver(client, sender.ID, &protobuf.SC_50008{Player: game.friendInfo(self)})
	}
	return client.Send(&protobuf.SC_50007{Result: result(err)})
}

// handleRejectFriendRequest answers RejectFriendRequestCommand (CS_50009):
// drops one sender's request, or all of them (id 0). The client drops them
// itself on result 0; the sender isn't told. Built from the Lua alone.
func (game *Game) handleRejectFriendRequest(client *connection.Client, req *protobuf.CS_50009) error {
	ctx := context.Background()
	var err error
	if req.GetId() == rejectAll {
		err = game.DB.DeleteFriendRequests(ctx, client.CommanderID)
	} else {
		var removed int64
		removed, err = game.DB.DeleteFriendRequest(ctx, db.DeleteFriendRequestParams{CommanderID: client.CommanderID, SenderID: req.GetId()})
		if err == nil && removed == 0 {
			err = errRefused
		}
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_50010{Result: result(err)})
}

// handleDeleteFriend answers DeleteFriendCommand (CS_50011): the friendship
// ends on both sides. The client waits for SC_50013 to drop the friend
// (FriendProxy:removeFriend), so it goes to both, the other side when it is
// online. Built from the Lua alone.
func (game *Game) handleDeleteFriend(client *connection.Client, req *protobuf.CS_50011) error {
	removed, err := game.DB.DeleteFriendship(context.Background(), db.DeleteFriendshipParams{CommanderID: client.CommanderID, FriendID: req.GetId()})
	if err != nil {
		return err
	}
	if removed == 0 {
		err = errNotFriend
	} else {
		game.deliver(client, client.CommanderID, &protobuf.SC_50013{Id: proto.Uint32(req.GetId())})
		game.deliver(client, req.GetId(), &protobuf.SC_50013{Id: proto.Uint32(client.CommanderID)})
	}
	return client.Send(&protobuf.SC_50012{Result: result(err)})
}

// handleSendFriendMessage answers SendFriendMsgCommand (CS_50105), a private
// message to a friend: pushed to it (SC_50104, FriendProxy counts it
// unread). Messages aren't kept: SC_50000 has none, and the client reads
// result 28 as friend_offline, so a friend offline gets nothing. The sender
// adds its own copy on result 0. Built from the Lua alone.
func (game *Game) handleSendFriendMessage(client *connection.Client, req *protobuf.CS_50105) error {
	ctx := context.Background()
	friend, err := game.DB.CountFriend(ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: req.GetId()})
	if err != nil {
		return err
	}
	if friend == 0 || req.GetContent() == "" {
		return client.Send(&protobuf.SC_50106{Result: result(errRefused)})
	}
	sender, err := game.DB.GetCommander(ctx, client.CommanderID)
	if err != nil {
		return err
	}
	msg := &protobuf.SC_50104{Msg: &protobuf.MSG_INFO_P50{
		Timestamp: proto.Uint32(uint32(time.Now().Unix())),
		Player:    game.playerInfoP50(sender),
		Content:   proto.String(req.GetContent()),
	}}
	if !game.deliver(client, req.GetId(), msg) {
		err = errNotFriend
	}
	return client.Send(&protobuf.SC_50106{Result: result(err)})
}

// handleBlacklist answers GetBlackListCommand (CS_50016, type 0), sent when
// the friend scene's blacklist page or a player's info opens: the players
// the commander blocked, oldest first. Built from the Lua alone.
func (game *Game) handleBlacklist(client *connection.Client, req *protobuf.CS_50016) error {
	blocked, err := game.DB.ListBlacklist(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_50017{}
	for _, row := range blocked {
		reply.BlackList = append(reply.BlackList, game.playerInfoP50(row.Commander))
	}
	return client.Send(reply)
}

// handleBlockPlayer answers AddBlackListCommand (CS_50109): the player joins
// the blacklist, at most MAX_BLACKLIST_COUNT. The client hides its world
// chat and friend requests itself and adds it to its list on result 0; it
// never blocks a friend. Built from the Lua alone.
func (game *Game) handleBlockPlayer(client *connection.Client, req *protobuf.CS_50109) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		if req.GetId() == client.CommanderID {
			return errRefused
		}
		if _, err := commanderByID(ctx, tx.Queries, req.GetId()); err != nil {
			return err
		}
		friend, err := tx.CountFriend(ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: req.GetId()})
		if err != nil {
			return err
		}
		if friend > 0 {
			return errRefused
		}
		blocked, err := tx.CountBlacklist(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if blocked >= maxBlacklist {
			return errLimitReached
		}
		return tx.AddBlacklist(ctx, db.AddBlacklistParams{CommanderID: client.CommanderID, BlockedID: req.GetId(), BlockedAt: time.Now().Unix()})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_50110{Result: result(err)})
}

// handleUnblockPlayer answers RelieveBlackListCommand (CS_50107): the player
// leaves the blacklist, the client drops it from its list on result 0.
// Built from the Lua alone.
func (game *Game) handleUnblockPlayer(client *connection.Client, req *protobuf.CS_50107) error {
	removed, err := game.DB.DeleteBlacklist(context.Background(), db.DeleteBlacklistParams{CommanderID: client.CommanderID, BlockedID: req.GetId()})
	if err != nil {
		return err
	}
	if removed == 0 {
		err = errRefused
	}
	return client.Send(&protobuf.SC_50108{Result: result(err)})
}
