package logging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

func (ms *MonitoringService) registerBackfillHandlers(serviceCtx context.Context, workload monitoringWorkloadState) {
	// Register one-shot entry/exit backfill handler (Option A).
	ms.router.RegisterHandler(TaskTypeMonitorBackfillEntryExitDay, func(ctx context.Context, payload any) error {
		p, ok := payload.(BackfillEntryExitDayPayload)
		if !ok {
			log.ErrorLoggerRaw().Error("Invalid payload type for "+TaskTypeMonitorBackfillEntryExitDay, "type", fmt.Sprintf("%T", payload))
			return nil
		}
		channelID := strings.TrimSpace(p.ChannelID)
		day := strings.TrimSpace(p.Day)
		if channelID == "" {
			return nil
		}
		if day == "" {
			day = time.Now().UTC().Format("2006-01-02")
		}

		start, err := time.Parse("2006-01-02", day)
		if err != nil {
			return nil
		}
		end := start.Add(24 * time.Hour)

		// Resolve guild ID from channel
		var guildID string
		if ms.session != nil && ms.session.State != nil {
			if ch, _ := ms.session.State.Channel(channelID); ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID == "" && ms.session != nil {
			if ch, err := ms.session.Channel(channelID); err == nil && ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID == "" {
			return nil
		}

		log.ApplicationLogger().Info("📥 Starting entry/exit backfill (day)", "channelID", channelID, "guildID", guildID, "day", day)

		botID := ""
		if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
			botID = ms.session.State.User.ID
		}

		var before string
		processedCount := 0
		eventsFound := 0
		startTime := time.Now()

		for {
			if err := serviceCtx.Err(); err != nil {
				return fmt.Errorf("MonitoringService.registerBackfillHandlers: %w", err)
			}
			msgs, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() ([]*discordgo.Message, error) {
				return ms.session.ChannelMessages(channelID, 100, before, "", "")
			})
			if err != nil {
				log.ErrorLoggerRaw().Error("Failed to fetch channel messages for backfill", "channelID", channelID, "err", err)
				break
			}
			if len(msgs) == 0 {
				break
			}

			// Messages come newest -> oldest
			stop := false
			for _, m := range msgs {
				if err := serviceCtx.Err(); err != nil {
					return fmt.Errorf("MonitoringService.registerBackfillHandlers: %w", err)
				}
				t := m.Timestamp.UTC()
				// Stop if we've paged past the target day
				if t.Before(start) {
					stop = true
					break
				}
				// Only consider messages within the day
				if t.Before(end) && !t.Before(start) {
					var rc files.RuntimeConfig
					if cfg := ms.configManager.Config(); cfg != nil {
						rc = cfg.ResolveRuntimeConfig(guildID)
					}
					evt, userID, ok := parseEntryExitBackfillMessage(m, botID, rc)
					if ok && ms.store != nil {
						eventsFound++
						if evt == "join" {
							if err := ms.store.UpsertMemberJoin(guildID, userID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(day): failed to persist member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
							}
							if err := ms.store.IncrementDailyMemberJoin(guildID, userID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(day): failed to increment daily member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
							}
						} else if evt == "leave" {
							// If name was not in message, check if still in server via code
							stillInServer := false
							if ms.session != nil {
								mem, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
									return ms.session.GuildMember(guildID, userID)
								})
								if err == nil && mem != nil {
									stillInServer = true
								}
							}

							if !stillInServer {
								if err := ms.store.IncrementDailyMemberLeave(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(day): failed to increment daily member leave", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
							}
						}
						// Record the oldest processed timestamp for this channel
						if err := ms.store.SetMetadata(serviceCtx, "backfill_progress:"+channelID, t); err != nil {
							log.ApplicationLogger().Warn("Backfill(day): failed to persist progress metadata", "guildID", guildID, "channelID", channelID, "at", t, "err", err)
						}
					}
				}
				processedCount++
			}

			if processedCount%500 == 0 || processedCount < 500 && processedCount%100 == 0 {
				log.ApplicationLogger().Info("⏳ Backfill in progress (day)...", "channelID", channelID, "processed", processedCount, "events_found", eventsFound)
			}

			// Prepare next page or stop
			before = msgs[len(msgs)-1].ID
			if stop {
				break
			}
		}

		log.ApplicationLogger().Info("✅ Backfill completed (day)", "channelID", channelID, "processed", processedCount, "events_found", eventsFound, "duration", time.Since(startTime).Round(time.Millisecond))
		return nil
	})

	// Register range-based entry/exit backfill handler (used for downtime recovery and historical scans)
	ms.router.RegisterHandler(TaskTypeMonitorBackfillEntryExitRange, func(ctx context.Context, payload any) error {
		p, ok := payload.(BackfillEntryExitRangePayload)
		if !ok {
			log.ErrorLoggerRaw().Error("Invalid payload type for "+TaskTypeMonitorBackfillEntryExitRange, "type", fmt.Sprintf("%T", payload))
			return nil
		}

		channelID := strings.TrimSpace(p.ChannelID)
		startRaw := strings.TrimSpace(p.Start)
		endRaw := strings.TrimSpace(p.End)
		if channelID == "" || startRaw == "" || endRaw == "" {
			log.ErrorLoggerRaw().Warn("Missing required fields for backfill range", "channelID", channelID, "start", startRaw, "end", endRaw)
			return nil
		}

		start, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			log.ErrorLoggerRaw().Error("Failed to parse start date for backfill range", "err", err, "start", startRaw)
			return nil
		}
		end, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			log.ErrorLoggerRaw().Error("Failed to parse end date for backfill range", "err", err, "end", endRaw)
			return nil
		}
		start = start.UTC()
		end = end.UTC()
		if !end.After(start) {
			log.ErrorLoggerRaw().Warn("End date must be after start date for backfill range", "start", start, "end", end)
			return nil
		}

		// Resolve guild ID from channel
		var guildID string
		if ms.session != nil && ms.session.State != nil {
			if ch, _ := ms.session.State.Channel(channelID); ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID == "" && ms.session != nil {
			if ch, err := ms.session.Channel(channelID); err == nil && ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID == "" {
			log.ErrorLoggerRaw().Warn("Could not resolve guild ID for channel during backfill", "channelID", channelID)
			return nil
		}

		log.ApplicationLogger().Info("📥 Starting entry/exit backfill (range)", "channelID", channelID, "guildID", guildID, "start", start.Format(time.RFC3339), "end", end.Format(time.RFC3339))

		botID := ""
		if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
			botID = ms.session.State.User.ID
		}

		var before string
		processedCount := 0
		eventsFound := 0
		startTime := time.Now()

		for {
			if err := serviceCtx.Err(); err != nil {
				return fmt.Errorf("MonitoringService.registerBackfillHandlers: %w", err)
			}
			msgs, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() ([]*discordgo.Message, error) {
				return ms.session.ChannelMessages(channelID, 100, before, "", "")
			})
			if err != nil {
				log.ErrorLoggerRaw().Error("Failed to fetch channel messages for backfill range", "channelID", channelID, "err", err)
				break
			}
			if len(msgs) == 0 {
				break
			}

			// Messages come newest -> oldest
			stop := false
			for _, m := range msgs {
				if err := serviceCtx.Err(); err != nil {
					return fmt.Errorf("MonitoringService.registerBackfillHandlers: %w", err)
				}
				t := m.Timestamp.UTC()
				// Stop if we've paged past the target window
				if t.Before(start) {
					stop = true
					break
				}
				// Only consider messages within the window
				if t.Before(end) && !t.Before(start) {
					var rc files.RuntimeConfig
					if cfg := ms.configManager.Config(); cfg != nil {
						rc = cfg.ResolveRuntimeConfig(guildID)
					}
					evt, userID, ok := parseEntryExitBackfillMessage(m, botID, rc)
					if ok && ms.store != nil {
						eventsFound++
						if evt == "join" {
							if err := ms.store.UpsertMemberJoin(guildID, userID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(range): failed to persist member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
							}
							if err := ms.store.IncrementDailyMemberJoin(guildID, userID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(range): failed to increment daily member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
							}
						} else if evt == "leave" {
							// If name was not in message, check if still in server via code
							stillInServer := false
							if ms.session != nil {
								mem, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
									return ms.session.GuildMember(guildID, userID)
								})
								if err == nil && mem != nil {
									stillInServer = true
								}
							}

							if !stillInServer {
								if err := ms.store.IncrementDailyMemberLeave(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(range): failed to increment daily member leave", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
							}
						}
						// Record the oldest processed timestamp for this channel
						if err := ms.store.SetMetadata(serviceCtx, "backfill_progress:"+channelID, t); err != nil {
							log.ApplicationLogger().Warn("Backfill(range): failed to persist progress metadata", "guildID", guildID, "channelID", channelID, "at", t, "err", err)
						}
					}
				}
				processedCount++
			}

			if processedCount%500 == 0 || processedCount < 500 && processedCount%100 == 0 {
				log.ApplicationLogger().Info("⏳ Backfill in progress (range)...", "channelID", channelID, "processed", processedCount, "events_found", eventsFound)
			}

			before = msgs[len(msgs)-1].ID
			if stop {
				break
			}
		}

		log.ApplicationLogger().Info("✅ Backfill completed (range)", "channelID", channelID, "processed", processedCount, "events_found", eventsFound, "duration", time.Since(startTime).Round(time.Millisecond))
		return nil
	})

	// Optionally auto-dispatch backfill tasks right after startup based on runtime config.
	//
	// Behavior:
	// - If `BackfillStartDay` is set: run day-based scan.
	// - Otherwise: if downtime is detected via the persisted last event timestamp and exceeds threshold, run a range scan to recover.
	//
	// New Condition: Backfill only runs if a channel is configured AND an initial start date is provided in config.
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		cfg := scopedCfg
		globalRC := cfg.RuntimeConfig

		// Get all potential channels and their resolved configs
		type backfillTarget struct {
			ChannelID      string
			RC             files.RuntimeConfig
			FeatureEnabled bool
		}
		targets := make([]backfillTarget, 0)

		// Global target if configured
		if globalRC.BackfillChannelID != "" {
			targets = append(targets, backfillTarget{
				ChannelID:      strings.TrimSpace(globalRC.BackfillChannelID),
				RC:             globalRC,
				FeatureEnabled: cfg.ResolveFeatures("").Backfill.Enabled,
			})
		}

		// Guild targets
		for _, g := range cfg.Guilds {
			cid := g.Channels.BackfillChannelID()
			if cid != "" {
				featureEnabled := cfg.ResolveFeatures(g.GuildID).Backfill.Enabled
				targets = append(targets, backfillTarget{
					ChannelID:      cid,
					RC:             cfg.ResolveRuntimeConfig(g.GuildID),
					FeatureEnabled: featureEnabled,
				})
			}
		}

		if len(targets) == 0 {
			log.ApplicationLogger().Debug("No target channels for backfill check")
		} else {
			lastEvent, hasLastEvent, err := ms.getLastEvent(serviceCtx)
			if err != nil {
				lastEvent = time.Time{}
				hasLastEvent = false
				log.ErrorLoggerRaw().Error(
					"Failed to read last event for backfill recovery; downtime recovery disabled for this startup",
					"operation", "monitoring.start.backfill.get_last_event",
					"err", err,
				)
			}
			now := time.Now().UTC()

			for _, target := range targets {
				cid := target.ChannelID
				rc := target.RC
				if !target.FeatureEnabled {
					log.ApplicationLogger().Debug("Backfill disabled by features.backfill.enabled", "channelID", cid)
					continue
				}
				day := strings.TrimSpace(rc.BackfillStartDay)
				initialDate := strings.TrimSpace(rc.BackfillInitialDate)

				if day != "" {
					dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
					err := ms.router.Dispatch(dispatchCtx, task.Task{
						Type:    TaskTypeMonitorBackfillEntryExitDay,
						Payload: BackfillEntryExitDayPayload{ChannelID: cid, Day: day},
						Options: task.TaskOptions{GroupKey: "backfill:" + cid},
					})
					cancel()
					if err != nil {
						log.ErrorLoggerRaw().Error("Failed to dispatch entry/exit backfill task (day)", "channelID", cid, "day", day, "err", err)
					} else {
						log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill task (day)", "channelID", cid, "day", day)
					}
					continue
				}

				// If no specific day, check for initial scan or recovery
				if initialDate == "" {
					log.ApplicationLogger().Debug("Backfill skip for channel: no day set and initial_date is empty", "channelID", cid)
					continue
				}

				// Check progress for this channel
				_, hasProgress, err := ms.store.Metadata(serviceCtx, "backfill_progress:"+cid)
				if err != nil {
					log.ErrorLoggerRaw().Error(
						"Failed to read backfill progress; skipping backfill dispatch for channel",
						"operation", "monitoring.start.backfill.get_progress",
						"channelID", cid,
						"err", err,
					)
					continue
				}

				if !hasProgress {
					// Use initialDate to calculate start date
					parsedDate, err := time.Parse("2006-01-02", initialDate)
					if err != nil {
						log.ApplicationLogger().Error("Failed to parse backfill_initial_date", "date", initialDate, "err", err)
						continue
					}
					start := parsedDate.Format(time.RFC3339)
					end := now.Format(time.RFC3339)
					dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
					err = ms.router.Dispatch(dispatchCtx, task.Task{
						Type:    TaskTypeMonitorBackfillEntryExitRange,
						Payload: BackfillEntryExitRangePayload{ChannelID: cid, Start: start, End: end},
						Options: task.TaskOptions{GroupKey: "backfill:" + cid},
					})
					cancel()
					if err != nil {
						log.ErrorLoggerRaw().Error("Failed to dispatch initial entry/exit backfill (range)", "channelID", cid, "start", start, "end", end, "err", err)
					} else {
						log.ApplicationLogger().Info("▶️ Dispatched initial entry/exit backfill (range)", "channelID", cid, "start", start)
					}
					continue
				}

				// If we have progress, check if we need downtime recovery
				if hasLastEvent {
					downtime := now.Sub(lastEvent)
					if downtime > downtimeThreshold {
						start := lastEvent.UTC().Format(time.RFC3339)
						end := now.Format(time.RFC3339)
						dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
						err := ms.router.Dispatch(dispatchCtx, task.Task{
							Type:    TaskTypeMonitorBackfillEntryExitRange,
							Payload: BackfillEntryExitRangePayload{ChannelID: cid, Start: start, End: end},
							Options: task.TaskOptions{GroupKey: "backfill:" + cid},
						})
						cancel()
						if err != nil {
							log.ErrorLoggerRaw().Error("Failed to dispatch entry/exit backfill recovery (range)", "channelID", cid, "start", start, "end", end, "err", err)
						} else {
							log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill recovery (range)", "channelID", cid, "start", start, "end", end)
						}
					} else {
						log.ApplicationLogger().Debug("Downtime below threshold, skipping recovery", "channelID", cid, "downtime", downtime)
					}
				} else {
					log.ApplicationLogger().Debug("No last event recorded, skipping downtime recovery", "channelID", cid)
				}
			}
		}
	} else {
		log.ApplicationLogger().Info("Backfill skip: config manager or config is nil")
	}
}
