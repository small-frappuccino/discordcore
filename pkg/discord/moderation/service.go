package moderation

import (
	"context"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

// Definimos os tipos de ações possíveis (Enum)
type ActionType uint8

const (
	ActionBan ActionType = iota
	ActionKick
	// ActionMute, ActionUnban etc... no futuro
)

// ModerationJob agora carrega a "Intenção" (Action)
type ModerationJob struct {
	Ctx          context.Context
	Bot          *app.BotInstance
	Action       ActionType
	TargetUserID string
	Reason       string
	// Propriedades específicas (o Worker ignora o que não for relevante para a Action)
	DeleteDays int
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.jobQueue:
			var err error

			// O Roteador Interno do Worker
			switch job.Action {
			case ActionBan:
				err = s.discord.ExecuteBan(job.Ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
			case ActionKick:
				err = s.discord.ExecuteKick(job.Ctx, job.Bot, job.TargetUserID, job.Reason)
			}

			if err != nil {
				slog.Error("Falha na tarefa de moderação",
					"action", job.Action,
					"guild_id", job.Bot.GuildID,
					"error", err)
			}
		}
	}
}

// O método de entrada genérico, que substitui o s.Ban()
func (s *Service) EnqueueTask(job ModerationJob) error {
	select {
	case s.jobQueue <- job:
		// SUCESSO: A tarefa entrou no funil!
		return nil
	default:
		// REJEIÇÃO TÁTICA (LOAD SHEDDING):
		// Se os administradores dispararam 10.000 bans, mas a fila só aguenta 1.000,
		// 9.000 comandos caem aqui instantaneamente em vez de quebrar a memória do bot.
		return ErrModerationQueueFull
	}
}
