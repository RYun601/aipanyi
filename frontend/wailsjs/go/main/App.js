// @ts-check
// AI盘译 前端绑定层（双模式）—— 由 scripts/gen-bindings.js 自动生成，请勿手工编辑。
//
// wails 原生生成的结果是 window['go'] 直连，只在桌面容器里有效；这里统一改写为
// 走 bridge.js 的 call()，由它按运行环境分发：
//   - 桌面模式（Wails）：转发到 window['go']['main']['App'][method]
//   - Web 模式（浏览器）：POST /api/call 反射派发
// 业务代码 import 方式不变。

import { call } from '../../bridge.js';
export function AbortChatWithAgent(...args) {
  return call('AbortChatWithAgent', args);
}

export function AbortSummaryStockNews(...args) {
  return call('AbortSummaryStockNews', args);
}

export function AddAllStockInfo(...args) {
  return call('AddAllStockInfo', args);
}

export function AddConcept(...args) {
  return call('AddConcept', args);
}

export function AddCronTask(...args) {
  return call('AddCronTask', args);
}

export function AddGroup(...args) {
  return call('AddGroup', args);
}

export function AddKBDocument(...args) {
  return call('AddKBDocument', args);
}

export function AddPrompt(...args) {
  return call('AddPrompt', args);
}

export function AddPromptTemplate(...args) {
  return call('AddPromptTemplate', args);
}

export function AddStockConcept(...args) {
  return call('AddStockConcept', args);
}

export function AddStockGroup(...args) {
  return call('AddStockGroup', args);
}

export function AddTradingRecord(...args) {
  return call('AddTradingRecord', args);
}

export function AnalyzeSentiment(...args) {
  return call('AnalyzeSentiment', args);
}

export function AnalyzeSentimentWithFreqWeight(...args) {
  return call('AnalyzeSentimentWithFreqWeight', args);
}

export function BatchDeleteAIResponseResult(...args) {
  return call('BatchDeleteAIResponseResult', args);
}

export function BatchDeleteAllStockInfo(...args) {
  return call('BatchDeleteAllStockInfo', args);
}

export function BuildKBGraph(...args) {
  return call('BuildKBGraph', args);
}

export function BuildTableXLSXBytes(...args) {
  return call('BuildTableXLSXBytes', args);
}

export function CalculateNextRunTime(...args) {
  return call('CalculateNextRunTime', args);
}

export function CalculateNextRunTimes(...args) {
  return call('CalculateNextRunTimes', args);
}

export function ChatWithAgent(...args) {
  return call('ChatWithAgent', args);
}

export function ChatWithAgentKBQA(...args) {
  return call('ChatWithAgentKBQA', args);
}

export function CheckDeviceBinding(...args) {
  return call('CheckDeviceBinding', args);
}

export function CheckFrequentTrading(...args) {
  return call('CheckFrequentTrading', args);
}

export function CheckSponsorCode(...args) {
  return call('CheckSponsorCode', args);
}

export function CheckStockBaseInfo(...args) {
  return call('CheckStockBaseInfo', args);
}

export function CheckUpdate(...args) {
  return call('CheckUpdate', args);
}

export function ClearAgentFeedback(...args) {
  return call('ClearAgentFeedback', args);
}

export function ClearSignalRecords(...args) {
  return call('ClearSignalRecords', args);
}

export function ClearUserProfile(...args) {
  return call('ClearUserProfile', args);
}

export function ClsCalendar(...args) {
  return call('ClsCalendar', args);
}

export function ConceptDetail(...args) {
  return call('ConceptDetail', args);
}

export function ConceptEventList(...args) {
  return call('ConceptEventList', args);
}

export function ConceptKLine(...args) {
  return call('ConceptKLine', args);
}

export function ConceptRealHead(...args) {
  return call('ConceptRealHead', args);
}

export function ConceptStocks(...args) {
  return call('ConceptStocks', args);
}

export function CreateCronTask(...args) {
  return call('CreateCronTask', args);
}

export function CreateKnowledgeBase(...args) {
  return call('CreateKnowledgeBase', args);
}

export function CreateMCPServer(...args) {
  return call('CreateMCPServer', args);
}

export function CreatePromptBacktestTask(...args) {
  return call('CreatePromptBacktestTask', args);
}

export function CreateSkill(...args) {
  return call('CreateSkill', args);
}

export function DelPrompt(...args) {
  return call('DelPrompt', args);
}

export function DeleteAIResponseResult(...args) {
  return call('DeleteAIResponseResult', args);
}

export function DeleteAgentFeedback(...args) {
  return call('DeleteAgentFeedback', args);
}

export function DeleteAiRecommendStocks(...args) {
  return call('DeleteAiRecommendStocks', args);
}

export function DeleteAllStockInfo(...args) {
  return call('DeleteAllStockInfo', args);
}

export function DeleteCronTask(...args) {
  return call('DeleteCronTask', args);
}

export function DeleteCustomStrategy(...args) {
  return call('DeleteCustomStrategy', args);
}

export function DeleteDailyOperationPlan(...args) {
  return call('DeleteDailyOperationPlan', args);
}

export function DeleteDailyReview(...args) {
  return call('DeleteDailyReview', args);
}

export function DeleteFilesystemSkill(...args) {
  return call('DeleteFilesystemSkill', args);
}

export function DeleteKBDocument(...args) {
  return call('DeleteKBDocument', args);
}

export function DeleteKBGraph(...args) {
  return call('DeleteKBGraph', args);
}

export function DeleteKnowledgeBase(...args) {
  return call('DeleteKnowledgeBase', args);
}

export function DeleteMCPServer(...args) {
  return call('DeleteMCPServer', args);
}

export function DeleteMorningStrategy(...args) {
  return call('DeleteMorningStrategy', args);
}

export function DeletePromptBacktestTask(...args) {
  return call('DeletePromptBacktestTask', args);
}

export function DeletePromptTemplate(...args) {
  return call('DeletePromptTemplate', args);
}

export function DeleteSkill(...args) {
  return call('DeleteSkill', args);
}

export function DeleteSkillFile(...args) {
  return call('DeleteSkillFile', args);
}

export function DeleteStockChangeHistory(...args) {
  return call('DeleteStockChangeHistory', args);
}

export function DeleteTradingRecord(...args) {
  return call('DeleteTradingRecord', args);
}

export function DisableFilesystemSkill(...args) {
  return call('DisableFilesystemSkill', args);
}

export function EMDictCode(...args) {
  return call('EMDictCode', args);
}

export function EnableCronTask(...args) {
  return call('EnableCronTask', args);
}

export function EnableFilesystemSkill(...args) {
  return call('EnableFilesystemSkill', args);
}

export function EnableMCPServer(...args) {
  return call('EnableMCPServer', args);
}

export function EnableSkill(...args) {
  return call('EnableSkill', args);
}

export function ExecuteCronTaskNow(...args) {
  return call('ExecuteCronTaskNow', args);
}

export function ExportConfig(...args) {
  return call('ExportConfig', args);
}

export function ExportConfigData(...args) {
  return call('ExportConfigData', args);
}

export function ExportTableToXLSX(...args) {
  return call('ExportTableToXLSX', args);
}

export function ExportTradingRecordTemplate(...args) {
  return call('ExportTradingRecordTemplate', args);
}

export function FetchAiModelInfo(...args) {
  return call('FetchAiModelInfo', args);
}

export function FetchAiModels(...args) {
  return call('FetchAiModels', args);
}

export function FetchAndSaveMarketStatistic(...args) {
  return call('FetchAndSaveMarketStatistic', args);
}

export function FindConceptCodeByName(...args) {
  return call('FindConceptCodeByName', args);
}

export function Follow(...args) {
  return call('Follow', args);
}

export function FollowFund(...args) {
  return call('FollowFund', args);
}

export function GenerateDailyReviewNow(...args) {
  return call('GenerateDailyReviewNow', args);
}

export function GenerateMorningStrategyNow(...args) {
  return call('GenerateMorningStrategyNow', args);
}

export function GetAIResponseResult(...args) {
  return call('GetAIResponseResult', args);
}

export function GetAIResponseResultList(...args) {
  return call('GetAIResponseResultList', args);
}

export function GetAgentFeedbackStats(...args) {
  return call('GetAgentFeedbackStats', args);
}

export function GetAiAssistantSession(...args) {
  return call('GetAiAssistantSession', args);
}

export function GetAiConfigs(...args) {
  return call('GetAiConfigs', args);
}

export function GetAiRecommendStocksList(...args) {
  return call('GetAiRecommendStocksList', args);
}

export function GetAiRecommendStocksTodayStats(...args) {
  return call('GetAiRecommendStocksTodayStats', args);
}

export function GetAllBKCodes(...args) {
  return call('GetAllBKCodes', args);
}

export function GetAllConceptCodes(...args) {
  return call('GetAllConceptCodes', args);
}

export function GetAllConceptPlates(...args) {
  return call('GetAllConceptPlates', args);
}

export function GetAllConcepts(...args) {
  return call('GetAllConcepts', args);
}

export function GetAllCustomStrategies(...args) {
  return call('GetAllCustomStrategies', args);
}

export function GetAllDeptPolicyNews(...args) {
  return call('GetAllDeptPolicyNews', args);
}

export function GetAllGroupStocks(...args) {
  return call('GetAllGroupStocks', args);
}

export function GetAllIndustries(...args) {
  return call('GetAllIndustries', args);
}

export function GetAllIndustryPlates(...args) {
  return call('GetAllIndustryPlates', args);
}

export function GetAllKBVectorizingStatuses(...args) {
  return call('GetAllKBVectorizingStatuses', args);
}

export function GetAllMCPTools(...args) {
  return call('GetAllMCPTools', args);
}

export function GetAllMarkets(...args) {
  return call('GetAllMarkets', args);
}

export function GetAllSkills(...args) {
  return call('GetAllSkills', args);
}

export function GetAllStockChangesWithPaging(...args) {
  return call('GetAllStockChangesWithPaging', args);
}

export function GetAllStockConcepts(...args) {
  return call('GetAllStockConcepts', args);
}

export function GetAllStockInfoById(...args) {
  return call('GetAllStockInfoById', args);
}

export function GetAllStockInfoList(...args) {
  return call('GetAllStockInfoList', args);
}

export function GetAllStocks(...args) {
  return call('GetAllStocks', args);
}

export function GetAllTdxTransactionData(...args) {
  return call('GetAllTdxTransactionData', args);
}

export function GetBKConstituentStocks(...args) {
  return call('GetBKConstituentStocks', args);
}

export function GetBKFundFlowList(...args) {
  return call('GetBKFundFlowList', args);
}

export function GetBKFundFlowListByDate(...args) {
  return call('GetBKFundFlowListByDate', args);
}

export function GetBKFundFlowTopList(...args) {
  return call('GetBKFundFlowTopList', args);
}

export function GetBKFundFlowTopListByDate(...args) {
  return call('GetBKFundFlowTopListByDate', args);
}

export function GetChangeRank(...args) {
  return call('GetChangeRank', args);
}

export function GetChangeTypeDailyStats(...args) {
  return call('GetChangeTypeDailyStats', args);
}

export function GetChipDistribution(...args) {
  return call('GetChipDistribution', args);
}

export function GetConceptFundFlowList(...args) {
  return call('GetConceptFundFlowList', args);
}

export function GetConceptFundFlowListByDate(...args) {
  return call('GetConceptFundFlowListByDate', args);
}

export function GetConceptFundFlowTopList(...args) {
  return call('GetConceptFundFlowTopList', args);
}

export function GetConceptFundFlowTopListByDate(...args) {
  return call('GetConceptFundFlowTopListByDate', args);
}

export function GetConceptList(...args) {
  return call('GetConceptList', args);
}

export function GetConfig(...args) {
  return call('GetConfig', args);
}

export function GetCronTaskByID(...args) {
  return call('GetCronTaskByID', args);
}

export function GetCronTaskList(...args) {
  return call('GetCronTaskList', args);
}

export function GetCronTaskTypes(...args) {
  return call('GetCronTaskTypes', args);
}

export function GetCustomStrategyList(...args) {
  return call('GetCustomStrategyList', args);
}

export function GetDailyChangeStats(...args) {
  return call('GetDailyChangeStats', args);
}

export function GetDailyDimensionStats(...args) {
  return call('GetDailyDimensionStats', args);
}

export function GetDailyOperationPlanByID(...args) {
  return call('GetDailyOperationPlanByID', args);
}

export function GetDailyOperationPlanList(...args) {
  return call('GetDailyOperationPlanList', args);
}

export function GetDailyReviewByDate(...args) {
  return call('GetDailyReviewByDate', args);
}

export function GetDailyReviewList(...args) {
  return call('GetDailyReviewList', args);
}

export function GetEffectiveSponsorVip(...args) {
  return call('GetEffectiveSponsorVip', args);
}

export function GetFeishuBotStatus(...args) {
  return call('GetFeishuBotStatus', args);
}

export function GetFollowList(...args) {
  return call('GetFollowList', args);
}

export function GetFollowedFund(...args) {
  return call('GetFollowedFund', args);
}

export function GetFollowedFundPaged(...args) {
  return call('GetFollowedFundPaged', args);
}

export function GetFundHistoryNetValue(...args) {
  return call('GetFundHistoryNetValue', args);
}

export function GetFundKLine(...args) {
  return call('GetFundKLine', args);
}

export function GetFundRanking(...args) {
  return call('GetFundRanking', args);
}

export function GetFundTop10Holdings(...args) {
  return call('GetFundTop10Holdings', args);
}

export function GetFuturesMemberRank(...args) {
  return call('GetFuturesMemberRank', args);
}

export function GetFuturesPositionTrend(...args) {
  return call('GetFuturesPositionTrend', args);
}

export function GetGlobalIndexTrend(...args) {
  return call('GetGlobalIndexTrend', args);
}

export function GetGovDepartments(...args) {
  return call('GetGovDepartments', args);
}

export function GetGroupList(...args) {
  return call('GetGroupList', args);
}

export function GetGroupStockList(...args) {
  return call('GetGroupStockList', args);
}

export function GetHistoryTdxMinuteTimeData(...args) {
  return call('GetHistoryTdxMinuteTimeData', args);
}

export function GetHistoryTdxTransactionData(...args) {
  return call('GetHistoryTdxTransactionData', args);
}

export function GetHotMoneySeats(...args) {
  return call('GetHotMoneySeats', args);
}

export function GetHotStrategy(...args) {
  return call('GetHotStrategy', args);
}

export function GetIndexQuotes(...args) {
  return call('GetIndexQuotes', args);
}

export function GetIndexTline(...args) {
  return call('GetIndexTline', args);
}

export function GetIndustryMoneyRankSina(...args) {
  return call('GetIndustryMoneyRankSina', args);
}

export function GetIndustryRank(...args) {
  return call('GetIndustryRank', args);
}

export function GetKBGraph(...args) {
  return call('GetKBGraph', args);
}

export function GetKBGraphBuildStatus(...args) {
  return call('GetKBGraphBuildStatus', args);
}

export function GetKBVectorizingStatus(...args) {
  return call('GetKBVectorizingStatus', args);
}

export function GetKeyDepartments(...args) {
  return call('GetKeyDepartments', args);
}

export function GetKeyDeptPolicyNews(...args) {
  return call('GetKeyDeptPolicyNews', args);
}

export function GetKnowledgeBase(...args) {
  return call('GetKnowledgeBase', args);
}

export function GetKoreaDayKLine(...args) {
  return call('GetKoreaDayKLine', args);
}

export function GetLatestDailyReview(...args) {
  return call('GetLatestDailyReview', args);
}

export function GetLatestMorningStrategy(...args) {
  return call('GetLatestMorningStrategy', args);
}

export function GetLatestTradingDay(...args) {
  return call('GetLatestTradingDay', args);
}

export function GetLhbDailySummary(...args) {
  return call('GetLhbDailySummary', args);
}

export function GetLhbSeatDetail(...args) {
  return call('GetLhbSeatDetail', args);
}

export function GetLongTermMemoryAiConfigId(...args) {
  return call('GetLongTermMemoryAiConfigId', args);
}

export function GetLongTermMemoryInfo(...args) {
  return call('GetLongTermMemoryInfo', args);
}

export function GetMCPServerByID(...args) {
  return call('GetMCPServerByID', args);
}

export function GetMCPServerList(...args) {
  return call('GetMCPServerList', args);
}

export function GetMCPToolsByServerID(...args) {
  return call('GetMCPToolsByServerID', args);
}

export function GetMachineId(...args) {
  return call('GetMachineId', args);
}

export function GetMarketEmotion(...args) {
  return call('GetMarketEmotion', args);
}

export function GetMarketStatisticByDate(...args) {
  return call('GetMarketStatisticByDate', args);
}

export function GetMoneyRankSina(...args) {
  return call('GetMoneyRankSina', args);
}

export function GetMorningStrategyByDate(...args) {
  return call('GetMorningStrategyByDate', args);
}

export function GetMorningStrategyList(...args) {
  return call('GetMorningStrategyList', args);
}

export function GetPolicyNews(...args) {
  return call('GetPolicyNews', args);
}

export function GetProfileLearnAiConfigId(...args) {
  return call('GetProfileLearnAiConfigId', args);
}

export function GetPromptBacktestPicks(...args) {
  return call('GetPromptBacktestPicks', args);
}

export function GetPromptBacktestTaskDetail(...args) {
  return call('GetPromptBacktestTaskDetail', args);
}

export function GetPromptBacktestTaskList(...args) {
  return call('GetPromptBacktestTaskList', args);
}

export function GetPromptTemplateBacktestDetail(...args) {
  return call('GetPromptTemplateBacktestDetail', args);
}

export function GetPromptTemplateBacktestStats(...args) {
  return call('GetPromptTemplateBacktestStats', args);
}

export function GetPromptTemplateList(...args) {
  return call('GetPromptTemplateList', args);
}

export function GetPromptTemplates(...args) {
  return call('GetPromptTemplates', args);
}

export function GetRecentDaysMarketStatistic(...args) {
  return call('GetRecentDaysMarketStatistic', args);
}

export function GetRecommendBacktestStats(...args) {
  return call('GetRecommendBacktestStats', args);
}

export function GetSectorAnchors(...args) {
  return call('GetSectorAnchors', args);
}

export function GetSignalRecordPage(...args) {
  return call('GetSignalRecordPage', args);
}

export function GetSignalStats(...args) {
  return call('GetSignalStats', args);
}

export function GetSkillByID(...args) {
  return call('GetSkillByID', args);
}

export function GetSkillList(...args) {
  return call('GetSkillList', args);
}

export function GetSponsorInfo(...args) {
  return call('GetSponsorInfo', args);
}

export function GetStockChangeHistory(...args) {
  return call('GetStockChangeHistory', args);
}

export function GetStockChanges(...args) {
  return call('GetStockChanges', args);
}

export function GetStockCommonKLine(...args) {
  return call('GetStockCommonKLine', args);
}

export function GetStockConceptsByStockCode(...args) {
  return call('GetStockConceptsByStockCode', args);
}

export function GetStockEastMoneyKLine(...args) {
  return call('GetStockEastMoneyKLine', args);
}

export function GetStockEastMoneyKLinePage(...args) {
  return call('GetStockEastMoneyKLinePage', args);
}

export function GetStockKLine(...args) {
  return call('GetStockKLine', args);
}

export function GetStockKLinePageWithFallback(...args) {
  return call('GetStockKLinePageWithFallback', args);
}

export function GetStockKLineWithFallback(...args) {
  return call('GetStockKLineWithFallback', args);
}

export function GetStockList(...args) {
  return call('GetStockList', args);
}

export function GetStockMinutePriceLineData(...args) {
  return call('GetStockMinutePriceLineData', args);
}

export function GetStockMoneyTrendByDay(...args) {
  return call('GetStockMoneyTrendByDay', args);
}

export function GetStockRealTimePrice(...args) {
  return call('GetStockRealTimePrice', args);
}

export function GetStoredPolicyNews(...args) {
  return call('GetStoredPolicyNews', args);
}

export function GetTdxCallAuction(...args) {
  return call('GetTdxCallAuction', args);
}

export function GetTdxCompanyCategoryContent(...args) {
  return call('GetTdxCompanyCategoryContent', args);
}

export function GetTdxCompanyCategoryList(...args) {
  return call('GetTdxCompanyCategoryList', args);
}

export function GetTdxCompanyInfo(...args) {
  return call('GetTdxCompanyInfo', args);
}

export function GetTdxFinanceInfo(...args) {
  return call('GetTdxFinanceInfo', args);
}

export function GetTdxMinuteTimeData(...args) {
  return call('GetTdxMinuteTimeData', args);
}

export function GetTdxSymbolBelongBoard(...args) {
  return call('GetTdxSymbolBelongBoard', args);
}

export function GetTdxTransactionData(...args) {
  return call('GetTdxTransactionData', args);
}

export function GetTdxXDXRInfo(...args) {
  return call('GetTdxXDXRInfo', args);
}

export function GetTelegraphList(...args) {
  return call('GetTelegraphList', args);
}

export function GetTimezone(...args) {
  return call('GetTimezone', args);
}

export function GetTodayMarketStatistic(...args) {
  return call('GetTodayMarketStatistic', args);
}

export function GetTradingRecordById(...args) {
  return call('GetTradingRecordById', args);
}

export function GetTradingRecordList(...args) {
  return call('GetTradingRecordList', args);
}

export function GetTradingRecordStatistics(...args) {
  return call('GetTradingRecordStatistics', args);
}

export function GetTypeStatsByDate(...args) {
  return call('GetTypeStatsByDate', args);
}

export function GetUplimitHot(...args) {
  return call('GetUplimitHot', args);
}

export function GetUserManual(...args) {
  return call('GetUserManual', args);
}

export function GetUserProfile(...args) {
  return call('GetUserProfile', args);
}

export function GetUserProfileEnabled(...args) {
  return call('GetUserProfileEnabled', args);
}

export function GetUserProfileSnapshot(...args) {
  return call('GetUserProfileSnapshot', args);
}

export function GetUserProfileUpdatedAt(...args) {
  return call('GetUserProfileUpdatedAt', args);
}

export function GetVersionInfo(...args) {
  return call('GetVersionInfo', args);
}

export function GetfundList(...args) {
  return call('GetfundList', args);
}

export function GlobalStockIndexes(...args) {
  return call('GlobalStockIndexes', args);
}

export function GlobalStockIndexesReadable(...args) {
  return call('GlobalStockIndexesReadable', args);
}

export function Greet(...args) {
  return call('Greet', args);
}

export function HideToTray(...args) {
  return call('HideToTray', args);
}

export function HotEvent(...args) {
  return call('HotEvent', args);
}

export function HotStock(...args) {
  return call('HotStock', args);
}

export function HotTopic(...args) {
  return call('HotTopic', args);
}

export function ImportSkillFromBase64(...args) {
  return call('ImportSkillFromBase64', args);
}

export function ImportSkillPackage(...args) {
  return call('ImportSkillPackage', args);
}

export function ImportSkillPackageFromPath(...args) {
  return call('ImportSkillPackageFromPath', args);
}

export function ImportTradingRecordsFromExcel(...args) {
  return call('ImportTradingRecordsFromExcel', args);
}

export function ImportTradingRecordsFromPath(...args) {
  return call('ImportTradingRecordsFromPath', args);
}

export function IndustryDetail(...args) {
  return call('IndustryDetail', args);
}

export function IndustryKLine(...args) {
  return call('IndustryKLine', args);
}

export function IndustryRealHead(...args) {
  return call('IndustryRealHead', args);
}

export function IndustryResearchReport(...args) {
  return call('IndustryResearchReport', args);
}

export function InitCronTasks(...args) {
  return call('InitCronTasks', args);
}

export function InitializeGroupSort(...args) {
  return call('InitializeGroupSort', args);
}

export function InvestCalendarTimeLine(...args) {
  return call('InvestCalendarTimeLine', args);
}

export function IsHKTradingTime(...args) {
  return call('IsHKTradingTime', args);
}

export function IsTradingDay(...args) {
  return call('IsTradingDay', args);
}

export function IsTradingTime(...args) {
  return call('IsTradingTime', args);
}

export function IsUSTradingTime(...args) {
  return call('IsUSTradingTime', args);
}

export function ListAIServicesForKB(...args) {
  return call('ListAIServicesForKB', args);
}

export function ListAgentFeedback(...args) {
  return call('ListAgentFeedback', args);
}

export function ListFilesystemSkills(...args) {
  return call('ListFilesystemSkills', args);
}

export function ListKBDocuments(...args) {
  return call('ListKBDocuments', args);
}

export function ListKBDocumentsPaged(...args) {
  return call('ListKBDocumentsPaged', args);
}

export function ListKnowledgeBases(...args) {
  return call('ListKnowledgeBases', args);
}

export function ListRecommendBacktest(...args) {
  return call('ListRecommendBacktest', args);
}

export function ListRecommendBacktestByPrompt(...args) {
  return call('ListRecommendBacktestByPrompt', args);
}

export function ListRecommendBacktestBySkill(...args) {
  return call('ListRecommendBacktestBySkill', args);
}

export function ListRecommendBacktestByTemplate(...args) {
  return call('ListRecommendBacktestByTemplate', args);
}

export function ListSkillFiles(...args) {
  return call('ListSkillFiles', args);
}

export function LongTigerRank(...args) {
  return call('LongTigerRank', args);
}

export function MarkdownContentForSave(...args) {
  return call('MarkdownContentForSave', args);
}

export function NewChatStream(...args) {
  return call('NewChatStream', args);
}

export function NewsPush(...args) {
  return call('NewsPush', args);
}

export function NotifySignal(...args) {
  return call('NotifySignal', args);
}

export function OpenURL(...args) {
  return call('OpenURL', args);
}

export function PackSkillToBase64(...args) {
  return call('PackSkillToBase64', args);
}

export function PickKBFilePath(...args) {
  return call('PickKBFilePath', args);
}

export function PickKBFilePaths(...args) {
  return call('PickKBFilePaths', args);
}

export function PromptPlazaRequest(...args) {
  return call('PromptPlazaRequest', args);
}

export function QuitApp(...args) {
  return call('QuitApp', args);
}

export function ReFleshTelegraphList(...args) {
  return call('ReFleshTelegraphList', args);
}

export function ReadSkillFile(...args) {
  return call('ReadSkillFile', args);
}

export function RefreshAllTdxTransactionData(...args) {
  return call('RefreshAllTdxTransactionData', args);
}

export function RefreshHistoryTdxTransactionData(...args) {
  return call('RefreshHistoryTdxTransactionData', args);
}

export function RefreshHotMoneySeats(...args) {
  return call('RefreshHotMoneySeats', args);
}

export function RelearnUserProfile(...args) {
  return call('RelearnUserProfile', args);
}

export function RemoveConcept(...args) {
  return call('RemoveConcept', args);
}

export function RemoveGroup(...args) {
  return call('RemoveGroup', args);
}

export function RemoveStockConcept(...args) {
  return call('RemoveStockConcept', args);
}

export function RemoveStockGroup(...args) {
  return call('RemoveStockGroup', args);
}

export function ResetHotMoneySeats(...args) {
  return call('ResetHotMoneySeats', args);
}

export function RestartAsAdmin(...args) {
  return call('RestartAsAdmin', args);
}

export function RunRecommendBacktest(...args) {
  return call('RunRecommendBacktest', args);
}

export function RzrqRank(...args) {
  return call('RzrqRank', args);
}

export function RzrqTrend(...args) {
  return call('RzrqTrend', args);
}

export function SaveAIResponseResult(...args) {
  return call('SaveAIResponseResult', args);
}

export function SaveAiAssistantSession(...args) {
  return call('SaveAiAssistantSession', args);
}

export function SaveAsMarkdown(...args) {
  return call('SaveAsMarkdown', args);
}

export function SaveCustomStrategy(...args) {
  return call('SaveCustomStrategy', args);
}

export function SaveDailyOperationPlan(...args) {
  return call('SaveDailyOperationPlan', args);
}

export function SaveHotMoneySeats(...args) {
  return call('SaveHotMoneySeats', args);
}

export function SaveImage(...args) {
  return call('SaveImage', args);
}

export function SaveKeyDepartments(...args) {
  return call('SaveKeyDepartments', args);
}

export function SaveSignalRecords(...args) {
  return call('SaveSignalRecords', args);
}

export function SaveStockChangesToHistory(...args) {
  return call('SaveStockChangesToHistory', args);
}

export function SaveUserProfile(...args) {
  return call('SaveUserProfile', args);
}

export function SaveWordFile(...args) {
  return call('SaveWordFile', args);
}

export function SearchAllKnowledge(...args) {
  return call('SearchAllKnowledge', args);
}

export function SearchCronTasks(...args) {
  return call('SearchCronTasks', args);
}

export function SearchFundCodes(...args) {
  return call('SearchFundCodes', args);
}

export function SearchKnowledgeBase(...args) {
  return call('SearchKnowledgeBase', args);
}

export function SearchLongTermMemory(...args) {
  return call('SearchLongTermMemory', args);
}

export function SearchStock(...args) {
  return call('SearchStock', args);
}

export function SendDingDingMessage(...args) {
  return call('SendDingDingMessage', args);
}

export function SendDingDingMessageByType(...args) {
  return call('SendDingDingMessageByType', args);
}

export function SendFeishuMessage(...args) {
  return call('SendFeishuMessage', args);
}

export function SendFeishuMessageByType(...args) {
  return call('SendFeishuMessageByType', args);
}

export function SetAlarmChangePercent(...args) {
  return call('SetAlarmChangePercent', args);
}

export function SetCostPriceAndVolume(...args) {
  return call('SetCostPriceAndVolume', args);
}

export function SetLongTermMemoryAiConfigId(...args) {
  return call('SetLongTermMemoryAiConfigId', args);
}

export function SetProfileLearnAiConfigId(...args) {
  return call('SetProfileLearnAiConfigId', args);
}

export function SetStockAICron(...args) {
  return call('SetStockAICron', args);
}

export function SetStockSort(...args) {
  return call('SetStockSort', args);
}

export function SetTradingPrice(...args) {
  return call('SetTradingPrice', args);
}

export function SetUserProfileEnabled(...args) {
  return call('SetUserProfileEnabled', args);
}

export function ShowFromTray(...args) {
  return call('ShowFromTray', args);
}

export function StartFeishuBot(...args) {
  return call('StartFeishuBot', args);
}

export function StartMCPOAuth(...args) {
  return call('StartMCPOAuth', args);
}

export function StockNotice(...args) {
  return call('StockNotice', args);
}

export function StockResearchReport(...args) {
  return call('StockResearchReport', args);
}

export function StopFeishuBot(...args) {
  return call('StopFeishuBot', args);
}

export function SubmitAgentFeedback(...args) {
  return call('SubmitAgentFeedback', args);
}

export function SummaryStockNews(...args) {
  return call('SummaryStockNews', args);
}

export function TestDingDingNotice(...args) {
  return call('TestDingDingNotice', args);
}

export function TestFeishuNotice(...args) {
  return call('TestFeishuNotice', args);
}

export function TestMCPServer(...args) {
  return call('TestMCPServer', args);
}

export function TradingRecordTemplateBytes(...args) {
  return call('TradingRecordTemplateBytes', args);
}

export function UnFollow(...args) {
  return call('UnFollow', args);
}

export function UnFollowFund(...args) {
  return call('UnFollowFund', args);
}

export function UpdateAiConfigs(...args) {
  return call('UpdateAiConfigs', args);
}

export function UpdateAiRecommendStocksAlert(...args) {
  return call('UpdateAiRecommendStocksAlert', args);
}

export function UpdateConcept(...args) {
  return call('UpdateConcept', args);
}

export function UpdateConfig(...args) {
  return call('UpdateConfig', args);
}

export function UpdateCronTask(...args) {
  return call('UpdateCronTask', args);
}

export function UpdateDailyOperationPlanAlert(...args) {
  return call('UpdateDailyOperationPlanAlert', args);
}

export function UpdateDailyOperationPlanStatus(...args) {
  return call('UpdateDailyOperationPlanStatus', args);
}

export function UpdateFilesystemSkillDescription(...args) {
  return call('UpdateFilesystemSkillDescription', args);
}

export function UpdateGroup(...args) {
  return call('UpdateGroup', args);
}

export function UpdateGroupSort(...args) {
  return call('UpdateGroupSort', args);
}

export function UpdateMCPServer(...args) {
  return call('UpdateMCPServer', args);
}

export function UpdatePromptTemplate(...args) {
  return call('UpdatePromptTemplate', args);
}

export function UpdateSkill(...args) {
  return call('UpdateSkill', args);
}

export function UpdateTradingRecord(...args) {
  return call('UpdateTradingRecord', args);
}

export function UploadImageToImageBed(...args) {
  return call('UploadImageToImageBed', args);
}

export function UploadKBFile(...args) {
  return call('UploadKBFile', args);
}

export function UploadKBFiles(...args) {
  return call('UploadKBFiles', args);
}

export function VacuumDatabase(...args) {
  return call('VacuumDatabase', args);
}

export function ValidateCronExpr(...args) {
  return call('ValidateCronExpr', args);
}

export function WriteSkillFile(...args) {
  return call('WriteSkillFile', args);
}

