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
	registerSpecialProjects(s)
	registerDebris(s)
	registerResearchPotential(s)
	registerNeighbors(s)
	registerCasusBelli(s)
	registerFleetPower(s)
	registerFleetTemplates(s)
	registerEconomyBreakdown(s)
	registerResourceMarket(s)
	registerPlanets(s)
	registerColonies(s)
	registerConstruction(s)
	registerArmies(s)
	registerTraditionsAscension(s)
	registerGovernance(s)
	registerNotifications(s)
	registerWars(s)
	registerLeaders(s)
	registerCouncil(s)
	registerEspionage(s)
	registerEspionageOperations(s)
	registerArchaeology(s)
	registerFirstContacts(s)
	registerStarbases(s)
	registerSectors(s)
	registerSystemConnections(s)
	registerEmpireAssets(s)
	registerGalacticCommunity(s)
	registerRawSave(s)
	registerSaveCatalog(s)
	registerSpecies(s)
	registerSpeciesRights(s)
	registerGrandArchive(s)
	registerRelics(s)
	registerFederations(s)
}
