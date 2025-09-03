package commands

/* func (ch *CommandHandler) buildCleanCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "clean",
		Description: "Bulk delete messages.",
		Handler:     ch.handleCleanCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "amount", Description: "Number of messages to delete (max 100)", Required: true},
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "(Optional) Only messages from this user", Required: false},
		},
	}
}

func (ch *CommandHandler) buildTimeoutCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "timeout",
		Description: "Timeout em um usuário (mute temporário).",
		Handler:     ch.handleTimeoutCommand,
		Options: []*discordgo.ApplicationCommandOption{
			// Aceita tanto seleção nativa quanto ID manual
			{
				Type:        discordgo.ApplicationCommandOptionUser,
				Name:        "user",
				Description: "Selecione o usuário ou insira o ID",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time",
				Description: "Tempo de timeout (ex: 10m, 1h, 1d)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "reason",
				Description: "Motivo do timeout",
				Required:    true,
			},
		},
	}
}

func (ch *CommandHandler) buildKickCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "kick",
		Description: "Kick a member from the server.",
		Handler:     ch.handleKickCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Selecione o usuário", Required: false},
			{Type: discordgo.ApplicationCommandOptionString, Name: "userid", Description: "ID do usuário (opcional)", Required: false},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for the kick", Required: false},
		},
	}
}

func (ch *CommandHandler) buildBanCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "ban",
		Description: "Ban a member from the server.",
		Handler:     ch.handleBanCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Selecione o usuário", Required: false},
			{Type: discordgo.ApplicationCommandOptionString, Name: "userid", Description: "ID do usuário (opcional)", Required: false},
			{Type: discordgo.ApplicationCommandOptionString, Name: "reason", Description: "Reason for the ban", Required: false},
		},
	}
}

func (ch *CommandHandler) buildUnbanCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "unban",
		Description: "Unban a user from the server.",
		Handler:     ch.handleUnbanCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "userid", Description: "ID of the user to unban", Required: true},
		},
	}
}

func (ch *CommandHandler) buildDeleteCommand() models.SlashCommand {
	return models.SlashCommand{
		Name:        "delete",
		Description: "Remove canais, categorias e cargos.",
		Handler:     ch.handleDeleteCommand,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "channel",
				Description: "Remove um canal por ID/menção",
				Options:     []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionChannel, Name: "id", Description: "Selecione o canal", Required: true}},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "category",
				Description: "Remove uma categoria por ID/menção",
				Options:     []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionChannel, Name: "id", Description: "Selecione a categoria", Required: true}},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "role",
				Description: "Remove um cargo por ID/menção/nome",
				Options:     []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionRole, Name: "role", Description: "Selecione o cargo", Required: true}},
			},
		},
	}
}*/

/* // Handler for the clean command
func (ch *CommandHandler) handleCleanCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var amount int
	var userID string
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "amount" {
			amount = int(opt.IntValue())
		}
	}

	// Ack imediato (público) para evitar timeout de 3s
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})

	if amount < 1 || amount > 100 {
		msg := "❌ Amount must be between 1 and 100."
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		go func() { time.Sleep(200 * time.Millisecond); _ = s.InteractionResponseDelete(i.Interaction) }()
		return
	}

	deleted, slowFallback, err := CleanMessages(s, i.ChannelID, amount, userID)
	if err != nil {
		msg := "❌ " + err.Error()
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		go func() { time.Sleep(1 * time.Second); _ = s.InteractionResponseDelete(i.Interaction) }()
		return
	}
	if deleted == 0 {
		msg := "ℹ️ No messages found to delete."
		_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
		go func() { time.Sleep(1 * time.Second); _ = s.InteractionResponseDelete(i.Interaction) }()
		return
	}

	if slowFallback {
		// Remover resposta pública e enviar um follow-up ephemeral
		_ = s.InteractionResponseDelete(i.Interaction)
		follow := &discordgo.WebhookParams{Content: fmt.Sprintf("✅ %d messages deleted. (using slow fallback due to >14d constraint)", deleted), Flags: discordgo.MessageFlagsEphemeral}
		_, _ = s.FollowupMessageCreate(i.Interaction, true, follow)
		return
	}

	// Caso normal: editar a resposta e apagar em 1s
	msg := fmt.Sprintf("✅ %d messages deleted.", deleted)
	_, _ = s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &msg})
	go func() { time.Sleep(3 * time.Second); _ = s.InteractionResponseDelete(i.Interaction) }()
}

// Handler for timeout command
func (ch *CommandHandler) handleTimeoutCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var userID, reason, timeStr string
	for _, opt := range i.ApplicationCommandData().Options {
		switch opt.Name {
		case "user":
			userID = opt.UserValue(s).ID
		case "userid":
			if userID == "" {
				userID = opt.StringValue()
			}
		case "time":
			timeStr = opt.StringValue()
		case "reason":
			reason = opt.StringValue()
		}
	}
	if userID == "" || timeStr == "" || reason == "" {
		ch.sendEphemeral(s, i, "Todos os argumentos são obrigatórios.")
		return
	}
	// Aqui você pode implementar a lógica de timeout usando a API do Discord
	// Exemplo: s.GuildMemberTimeout(i.GuildID, userID, duration, reason)
	ch.sendEphemeral(s, i, "Usuário <@"+userID+"> recebeu timeout por "+timeStr+". Motivo: "+reason)
}

// Handler for the kick command
func (ch *CommandHandler) handleKickCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var userID, reason string
	for _, opt := range options {
		if opt.Name == "user" && opt.UserValue(nil) != nil {
			userID = opt.UserValue(nil).ID
		}
		if opt.Name == "userid" && opt.StringValue() != "" {
			userID = opt.StringValue()
		}
		if opt.Name == "reason" && opt.StringValue() != "" {
			reason = opt.StringValue()
		}
	}
	if userID == "" {
		ch.respondWithError(s, i, "User not specified.")
		return
	}
	if reason == "" {
		reason = "No reason provided."
	}
	err := s.GuildMemberDeleteWithReason(i.GuildID, userID, reason)
	if err != nil {
		ch.respondWithError(s, i, "Failed to kick user: "+err.Error())
		return
	}
	ch.respondWithSuccess(s, i, "User kicked successfully.")
}

// Handler for the ban command
func (ch *CommandHandler) handleBanCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var userID, reason string
	for _, opt := range options {
		if opt.Name == "user" && opt.UserValue(nil) != nil {
			userID = opt.UserValue(nil).ID
		}
		if opt.Name == "userid" && opt.StringValue() != "" {
			userID = opt.StringValue()
		}
		if opt.Name == "reason" && opt.StringValue() != "" {
			reason = opt.StringValue()
		}
	}
	if userID == "" {
		ch.respondWithError(s, i, "User not specified.")
		return
	}
	if reason == "" {
		reason = "No reason provided."
	}
	err := s.GuildBanCreateWithReason(i.GuildID, userID, reason, 0)
	if err != nil {
		ch.respondWithError(s, i, "Failed to ban user: "+err.Error())
		return
	}
	ch.respondWithSuccess(s, i, "User banned successfully.")
}

// Handler for the unban command
func (ch *CommandHandler) handleUnbanCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	var userID string
	for _, opt := range options {
		if opt.Name == "userid" && opt.StringValue() != "" {
			userID = opt.StringValue()
		}
	}
	if userID == "" {
		ch.respondWithError(s, i, "User ID not specified.")
		return
	}
	err := s.GuildBanDelete(i.GuildID, userID)
	if err != nil {
		ch.respondWithError(s, i, "Failed to unban user: "+err.Error())
		return
	}
	ch.respondWithSuccess(s, i, "User unbanned successfully.")
}*/

/* // CleanMessages handles deletion of recent and old messages respecting Discord constraints
func CleanMessages(s *discordgo.Session, channelID string, amount int, userID string) (int, bool, error) {
	if amount <= 0 {
		return 0, false, nil
	}

	logger.WithFields(map[string]interface{}{
		"channelID": channelID,
		"amount":    amount,
		"userFilter": func() string {
			if userID == "" {
				return "(any)"
			}
			return userID
		}(),
	}).Info("clean: start")

	remaining := amount
	beforeID := ""
	deletedTotal := 0
	usedSlowFallback := false
	candidatesFound := 0
	page := 0

	for remaining > 0 {
		page++
		pageSize := 100

		logger.WithFields(map[string]interface{}{
			"page":      page,
			"pageSize":  pageSize,
			"beforeID":  beforeID,
			"remaining": remaining,
		}).Debug("clean: fetching page")

		msgs, err := s.ChannelMessages(channelID, pageSize, beforeID, "", "")
		if err != nil {
			return deletedTotal, usedSlowFallback, fmt.Errorf("failed to fetch messages: %w", err)
		}
		tmsgs := len(msgs)
		if tmsgs == 0 {
			logger.WithField("page", page).Debug("clean: no more messages")
			break
		}

		cutoff := time.Now().Add(-14 * 24 * time.Hour)
		var recent []string
		var older []string
		for _, msg := range msgs {
			// Ignore bot messages (inclui a resposta do /clean)
			if msg.Author != nil && msg.Author.ID == s.State.User.ID {
				continue
			}
			if userID != "" && (msg.Author == nil || msg.Author.ID != userID) {
				continue
			}
			candidatesFound++
			msgTime := msg.Timestamp
			if msgTime.Before(cutoff) {
				older = append(older, msg.ID)
			} else {
				recent = append(recent, msg.ID)
			}
		}

		logger.WithFields(map[string]interface{}{
			"page": page, "recent": len(recent), "older": len(older), "candidatesFound": candidatesFound,
		}).Debug("clean: candidates classified")

		if len(recent) == 0 && len(older) == 0 {
			beforeID = msgs[len(msgs)-1].ID
			logger.WithField("page", page).Debug("clean: no candidates this page; advancing")
			continue
		}

		// Limitar arrays ao restante necessário
		if len(recent) > remaining {
			recent = recent[:remaining]
		}

		deletedThisPage := 0

		// Bulk delete os recentes (<14 dias)
		if len(recent) == 1 {
			if err := s.ChannelMessageDelete(channelID, recent[0]); err == nil {
				deletedTotal++
				deletedThisPage++
				remaining--
			}
		} else if len(recent) > 1 {
			if err := s.ChannelMessagesBulkDelete(channelID, recent); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "too old") || strings.Contains(msg, "14") || strings.Contains(msg, "50034") {
					logger.WithField("reason", msg).Warn("clean: bulk delete failed; using slow fallback")
					usedSlowFallback = true
					for _, id := range recent {
						_ = s.ChannelMessageDelete(channelID, id)
						deletedTotal++
						deletedThisPage++
						remaining--
						time.Sleep(1 * time.Second)
					}
				} else {
					return deletedTotal, usedSlowFallback, fmt.Errorf("failed to bulk delete messages: %w", err)
				}
			} else {
				deletedTotal += len(recent)
				deletedThisPage += len(recent)
				remaining -= len(recent)
			}
		}

		// Mensagens antigas (>=14 dias) — sempre 1 por 1 com 1s
		if len(older) > remaining {
			older = older[:remaining]
		}
		for _, id := range older {
			if err := s.ChannelMessageDelete(channelID, id); err == nil {
				deletedTotal++
				deletedThisPage++
				remaining--
				usedSlowFallback = true
			}
			time.Sleep(1 * time.Second)
		}

		logger.WithFields(map[string]interface{}{
			"page": page, "deletedThisPage": deletedThisPage, "remaining": remaining,
		}).Info("clean: page deletion summary")

		// Preparar próxima página
		beforeID = msgs[len(msgs)-1].ID
	}

	if candidatesFound > 0 && deletedTotal == 0 {
		logger.WithField("candidatesFound", candidatesFound).Warn("clean: candidates found but 0 deleted")
		return 0, usedSlowFallback, fmt.Errorf("failed to delete messages (0/%d); check bot permissions", candidatesFound)
	}

	logger.WithFields(map[string]interface{}{
		"deletedTotal": deletedTotal, "usedSlowFallback": usedSlowFallback, "candidatesFound": candidatesFound,
	}).Info("clean: completed")

	return deletedTotal, usedSlowFallback, nil
} */
