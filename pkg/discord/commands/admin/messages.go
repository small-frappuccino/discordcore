package admin

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type adminMsgKey int

const (
	adminMsgNoMetrics adminMsgKey = iota
	adminMsgMetricsTitle
	adminMsgDefaultRoleCacheTTL
	adminMsgMetricsWatchStart
	adminMsgMetricsWatchTitle
	adminMsgNeedServiceName
	adminMsgServiceNotFound
	adminMsgStatusTitle
	adminMsgFieldState
	adminMsgFieldType
	adminMsgFieldPriority
	adminMsgFieldHealth
	adminMsgFieldUptime
	adminMsgFieldRestarts
	adminMsgFieldDependencies
	adminMsgFieldHealthIssue
	adminMsgFieldMetrics
	adminMsgServicesTitle
	adminMsgServicesDesc
	adminMsgRestartStarted
	adminMsgRestartFailed
	adminMsgRestartDone
	adminMsgHealthTitle
	adminMsgHealthDesc
	adminMsgFieldOverallStatus
	adminMsgFieldServices
	adminMsgServicesCount
	adminMsgFieldUnhealthyServices
	adminMsgSysInfoTitle
	adminMsgSysInfoDesc
	adminMsgFieldBot
	adminMsgFieldCore
	adminMsgFieldTotalServices
	adminMsgFieldRunningServices
	adminMsgHealthy
	adminMsgUnhealthy
	adminMsgAllSysOp
	adminMsgIssuesDetected
	adminMsgStateRunning
	adminMsgStateError
	adminMsgStateStopped
	adminMsgStateInitializing
	adminMsgStateStopping
	adminMsgStateUnknown
	adminMsgNotRunning
	numAdminMsgKeys
)

var adminCatalogLocales = []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR}

var adminCatalog = map[discordgo.Locale]map[adminMsgKey]string{
	discordgo.EnglishUS: {
		adminMsgNoMetrics:              "No service metrics are available right now. This reply stays private because these details are only useful for admin review.",
		adminMsgMetricsTitle:           "Service Metrics",
		adminMsgDefaultRoleCacheTTL:    "default (5m)",
		adminMsgMetricsWatchStart:      "Starting a live metrics watch for this channel. Updates will run every %ds for %ds.",
		adminMsgMetricsWatchTitle:      "Live Service Metrics",
		adminMsgNeedServiceName:        "This command needs a service name before it can continue, so this reply stays private.",
		adminMsgServiceNotFound:        "No service named %s was found, so this reply stays private.",
		adminMsgStatusTitle:            "Service Status: %s",
		adminMsgFieldState:             "State",
		adminMsgFieldType:              "Type",
		adminMsgFieldPriority:          "Priority",
		adminMsgFieldHealth:            "Health",
		adminMsgFieldUptime:            "Uptime",
		adminMsgFieldRestarts:          "Restarts",
		adminMsgFieldDependencies:      "Dependencies",
		adminMsgFieldHealthIssue:       "Health Issue",
		adminMsgFieldMetrics:           "Metrics",
		adminMsgServicesTitle:          "Registered Services",
		adminMsgServicesDesc:           "Here is the current service registry. This reply stays private because it is operational state. Total services: %d",
		adminMsgRestartStarted:         "Restarting service %s now. This reply stays private while the restart runs.",
		adminMsgRestartFailed:          "Service %s couldn't be restarted. This reply stays private because it includes internal service details: %v",
		adminMsgRestartDone:            "Service %s was restarted.",
		adminMsgHealthTitle:            "System Health Check",
		adminMsgHealthDesc:             "Here is the current system health snapshot. This reply stays private because it reflects internal service status.",
		adminMsgFieldOverallStatus:     "Overall Status",
		adminMsgFieldServices:          "Services",
		adminMsgServicesCount:          "%d/%d healthy",
		adminMsgFieldUnhealthyServices: "Unhealthy Services",
		adminMsgSysInfoTitle:           "System Information",
		adminMsgSysInfoDesc:            "Here is the current runtime and service summary. This reply stays private because it is operational data.",
		adminMsgFieldBot:               "Bot",
		adminMsgFieldCore:              "Core",
		adminMsgFieldTotalServices:     "Total Services",
		adminMsgFieldRunningServices:   "Running Services",
		adminMsgHealthy:                "Healthy",
		adminMsgUnhealthy:              "Unhealthy",
		adminMsgAllSysOp:               "All systems operational",
		adminMsgIssuesDetected:         "Issues detected",
		adminMsgStateRunning:           "Running",
		adminMsgStateError:             "Error",
		adminMsgStateStopped:           "Stopped",
		adminMsgStateInitializing:      "Initializing",
		adminMsgStateStopping:          "Stopping",
		adminMsgStateUnknown:           "Unknown",
		adminMsgNotRunning:             "Not running",
	},
	discordgo.PortugueseBR: {
		adminMsgNoMetrics:              "Nenhuma métrica de serviço está disponível agora. Esta resposta fica privada porque esses detalhes são úteis apenas para revisão administrativa.",
		adminMsgMetricsTitle:           "Métricas de Serviço",
		adminMsgDefaultRoleCacheTTL:    "padrão (5m)",
		adminMsgMetricsWatchStart:      "Iniciando monitoramento de métricas ao vivo neste canal. Atualizações a cada %ds por %ds.",
		adminMsgMetricsWatchTitle:      "Métricas de Serviço ao Vivo",
		adminMsgNeedServiceName:        "Este comando precisa de um nome de serviço para continuar. Esta resposta fica privada.",
		adminMsgServiceNotFound:        "Nenhum serviço chamado %s foi encontrado. Esta resposta fica privada.",
		adminMsgStatusTitle:            "Status do Serviço: %s",
		adminMsgFieldState:             "Estado",
		adminMsgFieldType:              "Tipo",
		adminMsgFieldPriority:          "Prioridade",
		adminMsgFieldHealth:            "Saúde",
		adminMsgFieldUptime:            "Uptime",
		adminMsgFieldRestarts:          "Reinicializações",
		adminMsgFieldDependencies:      "Dependências",
		adminMsgFieldHealthIssue:       "Problema de Saúde",
		adminMsgFieldMetrics:           "Métricas",
		adminMsgServicesTitle:          "Serviços Registrados",
		adminMsgServicesDesc:           "Este é o registro atual de serviços. Esta resposta fica privada porque é estado operacional. Total de serviços: %d",
		adminMsgRestartStarted:         "Reiniciando o serviço %s agora. Esta resposta fica privada enquanto a reinicialização ocorre.",
		adminMsgRestartFailed:          "O serviço %s não pôde ser reiniciado. Esta resposta fica privada porque inclui detalhes internos do serviço: %v",
		adminMsgRestartDone:            "O serviço %s foi reiniciado.",
		adminMsgHealthTitle:            "Verificação de Saúde do Sistema",
		adminMsgHealthDesc:             "Este é o snapshot atual de saúde do sistema. Esta resposta fica privada porque reflete o status interno dos serviços.",
		adminMsgFieldOverallStatus:     "Status Geral",
		adminMsgFieldServices:          "Serviços",
		adminMsgServicesCount:          "%d/%d saudáveis",
		adminMsgFieldUnhealthyServices: "Serviços com Problema",
		adminMsgSysInfoTitle:           "Informações do Sistema",
		adminMsgSysInfoDesc:            "Este é o resumo atual do tempo de execução e serviços. Esta resposta fica privada porque são dados operacionais.",
		adminMsgFieldBot:               "Bot",
		adminMsgFieldCore:              "Core",
		adminMsgFieldTotalServices:     "Total de Serviços",
		adminMsgFieldRunningServices:   "Serviços em Execução",
		adminMsgHealthy:                "Saudável",
		adminMsgUnhealthy:              "Com Problema",
		adminMsgAllSysOp:               "Todos os sistemas operacionais",
		adminMsgIssuesDetected:         "Problemas detectados",
		adminMsgStateRunning:           "Em Execução",
		adminMsgStateError:             "Erro",
		adminMsgStateStopped:           "Parado",
		adminMsgStateInitializing:      "Inicializando",
		adminMsgStateStopping:          "Parando",
		adminMsgStateUnknown:           "Desconhecido",
		adminMsgNotRunning:             "Não está em execução",
	},
}

func adminMsg(locale discordgo.Locale, key adminMsgKey, args ...any) string {
	msgs, ok := adminCatalog[locale]
	if !ok {
		msgs = adminCatalog[discordgo.EnglishUS]
	}
	template, ok := msgs[key]
	if !ok {
		template = adminCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}
