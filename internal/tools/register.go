package tools

import (
	"github.com/bmassemin/stellaris-mcp/internal/gamestate"
	"github.com/mark3labs/mcp-go/server"
)

func Register(s *server.MCPServer, dir string, localizer *gamestate.Localizer, flagScripts ...gamestate.FlagScriptSource) {
	var flagSource gamestate.FlagScriptSource
	if len(flagScripts) > 0 {
		flagSource = flagScripts[0]
	}
	register(s, dir, localizer, flagSource, nil, nil)
}

func RegisterWithInstalledScripts(s *server.MCPServer, dir string, localizer *gamestate.Localizer, localizationDir string) {
	register(s, dir, localizer, gamestate.NewFlagScriptSource(localizationDir), gamestate.NewTechnologyDefinitionSource(localizationDir),
		gamestate.NewTechnologyReferenceSource(localizationDir))
}

func register(s *server.MCPServer, dir string, localizer *gamestate.Localizer, flagSource gamestate.FlagScriptSource, technologySource gamestate.TechnologyDefinitionSource, technologyReferences gamestate.TechnologyReferenceSource) {
	saveDir = dir
	loc = localizer
	registerPing(s)
	registerEmpireOverview(s)
	registerResearchStatus(s)
	registerCompletedTechnologies(s)
	registerTechnologyDefinition(s, technologySource)
	registerTechnologyDefinitionTree(s, technologySource)
	registerTechnologyReferences(s, technologyReferences)
	registerSpecialProjects(s)
	registerEventChains(s)
	registerSavedEventTargets(s)
	registerPlayerEvents(s)
	registerSavedMessages(s)
	registerSavedFlags(s, flagSource)
	registerTradeRoutes(s)
	registerDebris(s)
	registerResearchPotential(s)
	registerNeighbors(s)
	registerCasusBelli(s)
	registerFleetPower(s)
	registerShipLoadout(s)
	registerShipDesignCollection(s)
	registerSpaceCombatObjects(s)
	registerFleetTemplates(s)
	registerEconomyBreakdown(s)
	registerResourceMarket(s)
	registerCountryObservationRefs(s)
	registerCountrySavedFields(s)
	registerControlledPlanets(s)
	registerCountryTimedModifiers(s)
	registerPlanets(s)
	registerPreFTL(s)
	registerColonies(s)
	registerConstruction(s)
	registerArmies(s)
	registerTraditionsAscension(s)
	registerGovernance(s)
	registerFactions(s)
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
	registerSystemContents(s)
	registerGalaxyMetadata(s)
	registerGalaxyRegions(s)
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
