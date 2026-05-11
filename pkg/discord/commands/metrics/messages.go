package metrics

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type metricsMsgKey int

const (
	metricsMsgRequiresServer metricsMsgKey = iota
	metricsMsgStoreUnavailable
	metricsMsgActivityLoadFailed
	metricsMsgActivityTitle
	metricsMsgChannelFilter
	metricsMsgActivityDesc
	metricsMsgFieldMsgChannels
	metricsMsgFieldMsgUsers
	metricsMsgFieldReactChannels
	metricsMsgFieldReactUsers
	metricsMsgNoData
	metricsMsgRange24h
	metricsMsgRange7d
	metricsMsgRange30d
	metricsMsgRange90d
	metricsMsgHealthTitle
	metricsMsgHealthDesc
	metricsMsgFieldCurrentMembers
	metricsMsgFieldCurrentMembersValue
	metricsMsgFieldJoinHistory
	metricsMsgFieldJoinHistoryValue
	metricsMsgFieldRetention
	metricsMsgFieldRetentionValue
	metricsMsgHealthFooter
	metricsMsgStatsTitle
	metricsMsgStatsDesc
	metricsMsgFieldJoined
	metricsMsgFieldJoinedValue
	metricsMsgFieldLeft
	metricsMsgFieldLeftValue
	metricsMsgFieldNet
	metricsMsgFieldNetValue
	metricsMsgBackfillNoRouter
	metricsMsgBackfillNoTaskRouter
	metricsMsgBackfillNoChannel
	metricsMsgBackfillInvalidDate
	metricsMsgBackfillDescDay
	metricsMsgBackfillDescRange
	metricsMsgBackfillDispatchFailed
	metricsMsgBackfillTitle
	metricsMsgBackfillDesc
	metricsMsgBackfillFooter
	numMetricsMsgKeys
)

var metricsCatalogLocales = []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR}

var metricsCatalog = map[discordgo.Locale]map[metricsMsgKey]string{
	discordgo.EnglishUS: {
		metricsMsgRequiresServer:           "This command only works inside a server, so this reply stays private.",
		metricsMsgStoreUnavailable:         "The metrics store couldn't be reached, so this reply stays private.",
		metricsMsgActivityLoadFailed:       "The activity metrics couldn't be loaded from the database right now, so this reply stays private. Try again shortly.",
		metricsMsgActivityTitle:            "Activity: %s%s",
		metricsMsgChannelFilter:            " in <#%s>",
		metricsMsgActivityDesc:             "Here is the recent message and reaction activity. This reply stays private because it is operational data.",
		metricsMsgFieldMsgChannels:         "Messages - Top Channels",
		metricsMsgFieldMsgUsers:            "Messages - Top Users",
		metricsMsgFieldReactChannels:       "Reactions - Top Channels",
		metricsMsgFieldReactUsers:          "Reactions - Top Users",
		metricsMsgNoData:                   "_no data_",
		metricsMsgRange24h:                 "Last 24h",
		metricsMsgRange7d:                  "Last 7d",
		metricsMsgRange30d:                 "Last 30d",
		metricsMsgRange90d:                 "Last 90d",
		metricsMsgHealthTitle:              "Server Health Stats",
		metricsMsgHealthDesc:               "Here is the current server health snapshot. This reply stays private because it combines database and cache data.\nDatabase size: `%s`",
		metricsMsgFieldCurrentMembers:      "Current Members",
		metricsMsgFieldCurrentMembersValue: "`%d` members currently in the server.",
		metricsMsgFieldJoinHistory:         "Join History",
		metricsMsgFieldJoinHistoryValue:    "`%s` unique users recorded in the database since tracking began.",
		metricsMsgFieldRetention:           "Retention",
		metricsMsgFieldRetentionValue:      "`%s` of historically recorded users are still in the server.",
		metricsMsgHealthFooter:             "Note: retention accuracy depends on the bot's member cache.",
		metricsMsgStatsTitle:               "Server Stats (%s)",
		metricsMsgStatsDesc:                "Here is the recent member movement snapshot. This reply stays private because it is operational data.",
		metricsMsgFieldJoined:              "Members Joined",
		metricsMsgFieldJoinedValue:         "`%s` joins in the last %s.",
		metricsMsgFieldLeft:                "Members Left",
		metricsMsgFieldLeftValue:           "`%s` leaves in the last %s.",
		metricsMsgFieldNet:                 "Net Growth",
		metricsMsgFieldNetValue:            "`%s` members.",
		metricsMsgBackfillNoRouter:         "The backfill couldn't start because the command router is unavailable. This reply stays private.",
		metricsMsgBackfillNoTaskRouter:     "The backfill couldn't start because the task router is unavailable. This reply stays private.",
		metricsMsgBackfillNoChannel:        "The backfill couldn't start because there is no channel selected or configured by default. This reply stays private.",
		metricsMsgBackfillInvalidDate:      "The backfill couldn't start because start_date must use YYYY-MM-DD. This reply stays private.",
		metricsMsgBackfillDescDay:          "Scanning channel <#%s> for day `%s`.",
		metricsMsgBackfillDescRange:        "Scanning channel <#%s> for the last `%d` days.",
		metricsMsgBackfillDispatchFailed:   "The backfill task couldn't be dispatched right now, so this reply stays private: %v",
		metricsMsgBackfillTitle:            "Backfill Started",
		metricsMsgBackfillDesc:             "The backfill request started. This reply stays private because it is an admin operation.\n%s",
		metricsMsgBackfillFooter:           "This process runs in the background. Use /metrics backfill-status to check progress.",
	},
	discordgo.PortugueseBR: {
		metricsMsgRequiresServer:           "Este comando só funciona dentro de um servidor. Esta resposta fica privada.",
		metricsMsgStoreUnavailable:         "Não foi possível acessar o banco de dados de métricas. Esta resposta fica privada.",
		metricsMsgActivityLoadFailed:       "As métricas de atividade não puderam ser carregadas no momento. Esta resposta fica privada. Tente novamente em breve.",
		metricsMsgActivityTitle:            "Atividade: %s%s",
		metricsMsgChannelFilter:            " em <#%s>",
		metricsMsgActivityDesc:             "Aqui está a atividade recente de mensagens e reações. Esta resposta fica privada porque é dado operacional.",
		metricsMsgFieldMsgChannels:         "Mensagens - Principais Canais",
		metricsMsgFieldMsgUsers:            "Mensagens - Principais Usuários",
		metricsMsgFieldReactChannels:       "Reações - Principais Canais",
		metricsMsgFieldReactUsers:          "Reações - Principais Usuários",
		metricsMsgNoData:                   "_sem dados_",
		metricsMsgRange24h:                 "Últimas 24h",
		metricsMsgRange7d:                  "Últimos 7d",
		metricsMsgRange30d:                 "Últimos 30d",
		metricsMsgRange90d:                 "Últimos 90d",
		metricsMsgHealthTitle:              "Estatísticas de Saúde do Servidor",
		metricsMsgHealthDesc:               "Aqui está o snapshot atual de saúde do servidor. Esta resposta fica privada porque combina dados do banco e do cache.\nTamanho do banco: `%s`",
		metricsMsgFieldCurrentMembers:      "Membros Atuais",
		metricsMsgFieldCurrentMembersValue: "`%d` membros atualmente no servidor.",
		metricsMsgFieldJoinHistory:         "Histórico de Entradas",
		metricsMsgFieldJoinHistoryValue:    "`%s` usuários únicos registrados no banco desde o início do rastreamento.",
		metricsMsgFieldRetention:           "Retenção",
		metricsMsgFieldRetentionValue:      "`%s` dos usuários registrados historicamente ainda estão no servidor.",
		metricsMsgHealthFooter:             "Nota: a precisão da retenção depende do cache de membros do bot.",
		metricsMsgStatsTitle:               "Estatísticas do Servidor (%s)",
		metricsMsgStatsDesc:                "Aqui está o snapshot recente de movimentação de membros. Esta resposta fica privada porque é dado operacional.",
		metricsMsgFieldJoined:              "Membros que Entraram",
		metricsMsgFieldJoinedValue:         "`%s` entradas nos últimos %s.",
		metricsMsgFieldLeft:                "Membros que Saíram",
		metricsMsgFieldLeftValue:           "`%s` saídas nos últimos %s.",
		metricsMsgFieldNet:                 "Crescimento Líquido",
		metricsMsgFieldNetValue:            "`%s` membros.",
		metricsMsgBackfillNoRouter:         "O backfill não pôde iniciar porque o roteador de comandos está indisponível. Esta resposta fica privada.",
		metricsMsgBackfillNoTaskRouter:     "O backfill não pôde iniciar porque o roteador de tarefas está indisponível. Esta resposta fica privada.",
		metricsMsgBackfillNoChannel:        "O backfill não pôde iniciar porque nenhum canal foi selecionado ou configurado como padrão. Esta resposta fica privada.",
		metricsMsgBackfillInvalidDate:      "O backfill não pôde iniciar porque start_date deve usar o formato AAAA-MM-DD. Esta resposta fica privada.",
		metricsMsgBackfillDescDay:          "Escaneando canal <#%s> para o dia `%s`.",
		metricsMsgBackfillDescRange:        "Escaneando canal <#%s> dos últimos `%d` dias.",
		metricsMsgBackfillDispatchFailed:   "A tarefa de backfill não pôde ser despachada agora. Esta resposta fica privada: %v",
		metricsMsgBackfillTitle:            "Backfill Iniciado",
		metricsMsgBackfillDesc:             "A solicitação de backfill foi iniciada. Esta resposta fica privada porque é uma operação administrativa.\n%s",
		metricsMsgBackfillFooter:           "Este processo roda em segundo plano. Use /metrics backfill-status para verificar o progresso.",
	},
}

func metricsMsg(locale discordgo.Locale, key metricsMsgKey, args ...any) string {
	msgs, ok := metricsCatalog[locale]
	if !ok {
		msgs = metricsCatalog[discordgo.EnglishUS]
	}
	s, ok := msgs[key]
	if !ok {
		s = metricsCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}
