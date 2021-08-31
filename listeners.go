package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func handleGiveawayReactions(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if !isThxMessage(r.MessageID) {
		return
	}

	if r.UserID == s.State.User.ID {
		return
	}

	member, err := s.GuildMember(r.GuildID, r.UserID)
	if err != nil {
		log.Panicln("("+r.GuildID+") "+"handleGiveawayReactions#s.GuildMember", err)
	}

	if hasAdminPermissions(member, r.GuildID) && (r.Emoji.Name == "✅" || r.Emoji.Name == "⛔") {
		reactionists, _ := s.MessageReactions(r.ChannelID, r.MessageID, "⛔", 10, "", "")
		for _, user := range reactionists {
			if user.ID == s.State.User.ID || (user.ID == r.UserID && r.MessageReaction.Emoji.Name == "⛔") {
				continue
			}
			err = s.MessageReactionRemove(r.ChannelID, r.MessageID, "⛔", user.ID)
			if err != nil {
				log.Println("("+r.GuildID+") "+"handleGiveawayReactions#s.MessageReactionRemove", err)
			}
		}
		reactionists, _ = s.MessageReactions(r.ChannelID, r.MessageID, "✅", 10, "", "")
		for _, user := range reactionists {
			if user.ID == s.State.User.ID || (user.ID == r.UserID && r.MessageReaction.Emoji.Name == "✅") {
				continue
			}
			err = s.MessageReactionRemove(r.ChannelID, r.MessageID, "✅", user.ID)
			if err != nil {
				log.Println("("+r.GuildID+") handleGiveawayReactions#s.MessageReactionRemove", err)
			}
		}
		participant := getParticipantByMessageId(r.MessageID)
		participant.AcceptTime.Time = time.Now()
		participant.AcceptTime.Valid = true
		participant.AcceptUser.String = member.User.Username
		participant.AcceptUser.Valid = true
		participant.AcceptUserId.String = r.UserID
		participant.AcceptUserId.Valid = true
		participant.IsAccepted.Valid = true
		if r.Emoji.Name == "✅" {
			log.Println(member.User.Username + "(" + member.User.ID + ") zaakceptował udział " + participant.UserName + "(" + participant.UserId + ") w giveawayu o ID " + fmt.Sprintf("%d", participant.GiveawayId))
			participant.IsAccepted.Bool = true
			_, err := DbMap.Update(participant)
			if err != nil {
				log.Panicln(err)
			}
			checkHelper(r.GuildID, participant.UserId)
			updateThxInfoMessage(&r.MessageID, r.GuildID, r.ChannelID, participant.UserId, participant.GiveawayId, &r.UserID, confirm)
		} else if r.Emoji.Name == "⛔" {
			log.Println(member.User.Username + "(" + member.User.ID + ") odrzucił udział " + participant.UserName + "(" + participant.UserId + ") w giveawayu o ID " + fmt.Sprintf("%d", participant.GiveawayId))
			participant.IsAccepted.Bool = false
			_, err := DbMap.Update(participant)
			if err != nil {
				log.Panicln("handleGiveawayReactions DbMap.Update(participant) " + err.Error())
			}
			checkHelper(r.GuildID, participant.UserId)
			updateThxInfoMessage(&r.MessageID, r.GuildID, r.ChannelID, participant.UserId, participant.GiveawayId, &r.UserID, reject)
		}
		return
	} else {
		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
	}
}

func HandleThxmeReactions(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if !isThxmeMessage(r.MessageID) {
		return
	}

	if r.UserID == s.State.User.ID {
		return
	}

	candidate := getParticipantCandidateByMessageId(r.MessageID)

	if r.UserID != candidate.CandidateApproverId {
		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.Name, r.UserID)
		return
	}

	reactionists, _ := s.MessageReactions(r.ChannelID, r.MessageID, "⛔", 10, "", "")
	for _, user := range reactionists {
		if user.ID == session.State.User.ID || (user.ID == r.UserID && r.MessageReaction.Emoji.Name == "⛔") {
			continue
		}

		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, "⛔", user.ID)
	}
	reactionists, _ = session.MessageReactions(r.ChannelID, r.MessageID, "✅", 10, "", "")
	for _, user := range reactionists {
		if user.ID == session.State.User.ID || (user.ID == r.UserID && r.MessageReaction.Emoji.Name == "✅") {
			continue
		}

		_ = s.MessageReactionRemove(r.ChannelID, r.MessageID, "✅", user.ID)
	}

	candidate.AcceptTime.Time = time.Now()
	candidate.AcceptTime.Valid = true
	candidate.IsAccepted.Valid = true

	if r.Emoji.Name == "✅" {
		if !candidate.IsAccepted.Valid {
			return
		}
		log.Println(candidate.CandidateApproverName + "(" + candidate.CandidateApproverId + ") zaakceptował prosbe o thx uzytkownika " + candidate.CandidateName + "(" + candidate.CandidateId + ")")
		_ = session.ChannelMessageDelete(candidate.ChannelId, candidate.MessageId)
		candidate.IsAccepted.Bool = true
		_, err := DbMap.Update(candidate)
		if err != nil {
			log.Panicln("handleGiveawayReactions DbMap.Update(participant) " + err.Error())
		}

		giveaway := getGiveawayForGuild(candidate.GuildId)
		if giveaway == nil {
			log.Println("(" + r.GuildID + ") handleThxmeReactions#getGiveawayForGuild")
			return
		}
		channelId := candidate.ChannelId
		participant := &Participant{
			UserId:     candidate.CandidateId,
			UserName:   candidate.CandidateName,
			GiveawayId: giveaway.Id,
			CreateTime: time.Now(),
			GuildId:    candidate.GuildId,
			GuildName:  candidate.GuildName,
			ChannelId:  channelId,
		}
		participant.MessageId = *updateThxInfoMessage(nil, r.GuildID, channelId, candidate.CandidateId, participant.GiveawayId, nil, wait)
		err = DbMap.Insert(participant)
		if err != nil {
			_, err = session.ChannelMessageSend(channelId, "Coś poszło nie tak przy dodawaniu podziękowania :(")
			if err != nil {
				log.Println("("+r.GuildID+") Could not send message to channel ("+r.ChannelID+")", err)
				return
			}

			log.Println("("+r.GuildID+") "+"handleGiveawayReactions#DbMap.Insert(participant)", err)
		}
		for err = session.MessageReactionAdd(channelId, participant.MessageId, "✅"); err != nil; err = session.MessageReactionAdd(channelId, participant.MessageId, "✅") {
		}
		for err = session.MessageReactionAdd(channelId, participant.MessageId, "⛔"); err != nil; err = session.MessageReactionAdd(channelId, participant.MessageId, "⛔") {
		}
	} else if r.Emoji.Name == "⛔" {
		log.Println(candidate.CandidateApproverName + " (" + candidate.CandidateApproverId + ") has rejected " + candidate.CandidateName + " (" + candidate.CandidateId + ") thank")
		candidate.IsAccepted.Bool = false
		_, err := DbMap.Update(candidate)
		if err != nil {
			log.Panicln("("+r.GuildID+") "+"handleGiveawayReactions#DbMap.Update(participant)", err)
		}
	}
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Author.Bot {
		return
	}

	if m.GuildID == "" {
		return
	}

	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	if m.Content == "!" {
		return
	}

	m.Content = m.Content[1:]
	args := strings.Fields(m.Content)
	switch args[0] {
	case "thx":
		handleThxCommand(m, args)
	case "thxme":
		handleThxmeCommand(s, m, args)
	case "giveaway":
		printGiveawayInfo(m.ChannelID, m.GuildID)
	case "csrvbot":
		handleCsrvbotCommand(s, m, args)
	case "doc":
		handleDocCommand(s, m, args)
	}
}

func onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	log.Println("Registered guild (" + g.Name + "#" + g.ID + ")")
	createConfigurationIfNotExists(g.Guild.ID)
	createMissingGiveaways(g.Guild)
	updateAllMembersSavedRoles(g.Guild.ID)
	checkHelpers(g.Guild.ID)
}

func onGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.GuildID == "" {
		return
	}
	updateMemberSavedRoles(m.Member, m.GuildID)
}

func onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.GuildID == "" {
		return
	}
	restoreMemberRoles(m.Member, m.GuildID)
}
