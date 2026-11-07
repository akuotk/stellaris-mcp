package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func Register(s *server.MCPServer, dir string, localizer *gamestate.Localizer) {
	saveDir = dir
	loc = localizer
	registerPing(s)
	registerEmpireOverview(s)
	registerResearchStatus(s)
	registerResearchPotential(s)
	registerNeighbors(s)
	registerFleetPower(s)
	registerEconomyBreakdown(s)
	registerResourceMarket(s)
	registerPlanets(s)
	registerColonies(s)
	registerConstruction(s)
	registerArmies(s)
	registerTraditionsAscension(s)
	registerGovernance(s)
	registerNotifications(s)
	registerLeaders(s)
	registerEspionage(s)
	registerEspionageOperations(s)
	registerArchaeology(s)
	registerFirstContacts(s)
	registerStarbases(s)
	registerEmpireAssets(s)
	registerGalacticCommunity(s)
	registerRawSave(s)
	registerSaveCatalog(s)
	registerSpecies(s)
	registerFederations(s)
}
