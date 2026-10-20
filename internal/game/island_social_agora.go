package game

import (
	"context"
	"slices"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The plaza (Get-Together Island, IslandAgoraAgency, mod/island/agora) and
// the island chat.

// islandChatMax is the longest island chat message kept, in characters.
// GUESS: the client's input field limits it already.
const islandChatMax = 200

// handleIslandAgoraUpgrade answers IslandUpgradeAgoraCommand (CS_21305,
// type 0): the plaza's next level, for the cost island_build_expansion
// lists for the current one (IslandAgoraAgency.GetUpgradeConsume, an island
// item the client takes itself: GAME.CONSUME_ITEM). The levels are
// IslandConst.AGORA_LEVEL_2_SIZE's, one more than the costs.
func (game *Game) handleIslandAgoraUpgrade(client *connection.Client, req *protobuf.CS_21305) error {
	reply := &protobuf.SC_21306{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		public := state.public()
		level := public.GetAgoraLevel()
		costs := islandSocialConfig.agoraCost
		if level == 0 || int(level) > len(costs) {
			reply.Result = result(errRefused)
			return nil
		}
		cost := costs[level-1]
		if cost[0] != dropTypeIslandItem {
			reply.Result = result(errRefused)
			return nil
		}
		if err := state.takeIslandItem(cost[1], cost[2]); err != nil {
			reply.Result = result(err)
			return nil
		}
		public.AgoraLevel = proto.Uint32(level + 1)
		reply.Result = result(nil)
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandAgoraSave answers IslandSaveAgoraCommand (CS_21307): the
// plaza's furniture, floor and tiles as placed (AgoraCalc.Encode*), the
// island's public placed_data. The visitors on the island see it change
// (SC_21309, IslandPlayerDataMonitor.HandleAgoraData). GUESS: what is placed
// isn't checked against the furniture owned, the client only places those.
func (game *Game) handleIslandAgoraSave(client *connection.Client, req *protobuf.CS_21307) error {
	data := req.GetUpdateData()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.island()
		if err != nil {
			return err
		}
		state.public().PlacedData = data
		return nil
	})
	if err != nil {
		return err
	}
	game.tellIsland(client, client.CommanderID, &protobuf.SC_21309{IslandId: proto.Uint32(client.CommanderID), UpdateData: data})
	return client.Send(&protobuf.SC_21308{Result: result(nil)})
}

// handleIslandAgoraSaveTheme answers IslandSaveAgoraThemeCommand (CS_21317):
// the plaza as it is, saved in a custom theme slot (Agora.GetUseableThemeId:
// 1..build_self_theme_num), replacing the one there. Deleting a theme
// (IslandDelAgoraThemeCommand) never reaches the server: a deleted theme's
// slot is taken again by the next one saved.
func (game *Game) handleIslandAgoraSaveTheme(client *connection.Client, req *protobuf.CS_21317) error {
	theme := req.GetTheme()
	reply := &protobuf.SC_21318{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if theme.GetId() == 0 || theme.GetId() > islandSocialConfig.themeMax {
			reply.Result = result(errRefused)
			return nil
		}
		state, err := tx.island()
		if err != nil {
			return err
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		raw, err := proto.Marshal(theme)
		if err != nil {
			return err
		}
		if social.Themes == nil {
			social.Themes = map[uint32][]byte{}
		}
		social.Themes[theme.GetId()] = raw
		reply.Result = result(nil)
		return state.setSocial(social)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandAgoraThemes answers IslandGetAgoraThemesCommand (CS_21321, id
// the island's): the custom plaza themes saved, once per session.
func (game *Game) handleIslandAgoraThemes(client *connection.Client, req *protobuf.CS_21321) error {
	reply := &protobuf.SC_21322{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := tx.otherIsland(req.GetId())
		if err != nil || state == nil {
			return err
		}
		social, err := state.social()
		if err != nil {
			return err
		}
		ids := make([]uint32, 0, len(social.Themes))
		for id := range social.Themes {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			theme := &protobuf.PB_PLACEMENT_THEME{}
			if err := proto.Unmarshal(social.Themes[id], theme); err != nil {
				return err
			}
			reply.ThemeList = append(reply.ThemeList, theme)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleIslandChat answers IslandSendChatCommand's island channel
// (CS_21323): the message goes to everyone on the island the commander is
// on, the commander included (SC_21325: IslandPlayerDataMonitor.AddChatMsg
// shows the sender's own message from it). The client already filtered the
// words and keeps the 10 s between messages. A refusal's tip is shown as
// is.
//
//belfast:static
func (game *Game) handleIslandChat(client *connection.Client, req *protobuf.CS_21323) error {
	islandID, here := game.islandOf(client)
	content := req.GetContent()
	if !here || islandID != req.GetIslandId() || content == "" || utf8.RuneCountInString(content) > islandChatMax {
		return client.Send(&protobuf.SC_21324{Result: result(errRefused), Tip: proto.String("")})
	}
	message := &protobuf.SC_21325{IslandId: proto.Uint32(islandID), UserId: proto.Uint32(client.CommanderID), Content: proto.String(content)}
	game.tellIsland(client, islandID, message)
	if err := client.Push(message); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_21324{Result: result(nil), Tip: proto.String("")})
}
