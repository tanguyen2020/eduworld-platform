// Package authz holds the permission catalog, system roles, Principal, scope constructors and Require
// (F002 US2/US3, ADR-0066, ADR-0072, ADR-0089 §3). The list of permissions mirrors ADMIN_PORTAL §2.2
// (the owning document, R33); TestCatalogMatchesAdminPortalDoc keeps both in sync.
package authz

import "strings"

// Permission is a code of the permission catalog (ADMIN_PORTAL §2.2, F002 T032).
type Permission string

// Permission catalog.
const (
	PermAnalyticsClassRead                       Permission = "analytics.class.read"
	PermAnalyticsSchoolRead                      Permission = "analytics.school.read"
	PermAnalyticsTenantRead                      Permission = "analytics.tenant.read"
	PermChatPolicyToggle                         Permission = "chat.policy.toggle"
	PermChatSensitiveTermsRead                   Permission = "chat.sensitive_terms.read"
	PermChatSensitiveTermsWrite                  Permission = "chat.sensitive_terms.write"
	PermChatTenantCategoryToggle                 Permission = "chat.tenant_category.toggle"
	PermChatTenantTermsWrite                     Permission = "chat.tenant_terms.write"
	PermCompetitionConfigLeaderboardWrite        Permission = "competition.config.leaderboard.write"
	PermCompetitionConfigMatchmakingWrite        Permission = "competition.config.matchmaking.write"
	PermCompetitionConfigRankingWrite            Permission = "competition.config.ranking.write"
	PermCompetitionConfigRewardWrite             Permission = "competition.config.reward.write"
	PermCompetitionConfigSeasonWrite             Permission = "competition.config.season.write"
	PermCompetitionConfigTournamentTemplateWrite Permission = "competition.config.tournament_template.write"
	PermContentReleaseRollback                   Permission = "content.release.rollback"
	PermCurriculumPublish                        Permission = "curriculum.publish"
	PermCurriculumRead                           Permission = "curriculum.read"
	PermCurriculumWrite                          Permission = "curriculum.write"
	PermGameAchievementWrite                     Permission = "game.achievement.write"
	PermGameAnalyticsRead                        Permission = "game.analytics.read"
	PermGameBossWrite                            Permission = "game.boss.write"
	PermGameChallengeWrite                       Permission = "game.challenge.write"
	PermGameChatAssetsWrite                      Permission = "game.chat_assets.write"
	PermGameCollectionWrite                      Permission = "game.collection.write"
	PermGameContentPublish                       Permission = "game.content.publish"
	PermGameContentReview                        Permission = "game.content.review"
	PermGameContentTranslate                     Permission = "game.content.translate"
	PermGameEconomyAdjust                        Permission = "game.economy.adjust"
	PermGameEconomyWrite                         Permission = "game.economy.write"
	PermGameEventOperate                         Permission = "game.event.operate"
	PermGameEventWrite                           Permission = "game.event.write"
	PermGameItemWrite                            Permission = "game.item.write"
	PermGameLiveopsRead                          Permission = "game.liveops.read"
	PermGameMinigameWrite                        Permission = "game.minigame.write"
	PermGameNpcWrite                             Permission = "game.npc.write"
	PermGameProgressionWrite                     Permission = "game.progression.write"
	PermGameQuestWrite                           Permission = "game.quest.write"
	PermGameRewardRuleWrite                      Permission = "game.reward_rule.write"
	PermGameSeasonTrackWrite                     Permission = "game.season_track.write"
	PermGameShopWrite                            Permission = "game.shop.write"
	PermGameWorldWrite                           Permission = "game.world.write"
	PermGameWorldEventWrite                      Permission = "game.world_event.write"
	PermLearningContentActivityReview            Permission = "learning_content.activity.review"
	PermLearningContentActivityWrite             Permission = "learning_content.activity.write"
	PermLearningContentImport                    Permission = "learning_content.import"
	PermLearningContentPublish                   Permission = "learning_content.publish"
	PermLearningContentReEvaluate                Permission = "learning_content.re_evaluate"
	PermLearningContentRead                      Permission = "learning_content.read"
	PermLearningContentReview                    Permission = "learning_content.review"
	PermLearningContentTranslate                 Permission = "learning_content.translate"
	PermLearningContentWrite                     Permission = "learning_content.write"
	PermLmsAssignmentRead                        Permission = "lms.assignment.read"
	PermLmsAssignmentWrite                       Permission = "lms.assignment.write"
	PermLmsChallengeWrite                        Permission = "lms.challenge.write"
	PermLmsClassRead                             Permission = "lms.class.read"
	PermLmsCourseWrite                           Permission = "lms.course.write"
	PermLmsLearningPathWrite                     Permission = "lms.learning_path.write"
	PermLmsReportRead                            Permission = "lms.report.read"
	PermLmsResultRelease                         Permission = "lms.result.release"
	PermLmsScheduleWrite                         Permission = "lms.schedule.write"
	PermLmsSchoolEventWrite                      Permission = "lms.school_event.write"
	PermLmsStudentPasswordReset                  Permission = "lms.student.password_reset"
	PermLmsStudentActivityRead                   Permission = "lms.student_activity.read"
	PermLmsStudyGroupWrite                       Permission = "lms.study_group.write"
	PermMediaUpload                              Permission = "media.upload"
	PermModerationCaseAct                        Permission = "moderation.case.act"
	PermModerationCaseLead                       Permission = "moderation.case.lead"
	PermModerationCaseMessagesRead               Permission = "moderation.case.messages.read"
	PermModerationCaseRead                       Permission = "moderation.case.read"
	PermModerationSanctionRead                   Permission = "moderation.sanction.read"
	PermModerationSanctionWrite                  Permission = "moderation.sanction.write"
	PermModerationSchoolNotificationAcknowledge  Permission = "moderation.school_notification.acknowledge"
	PermModerationSchoolNotificationRead         Permission = "moderation.school_notification.read"
	PermModerationSchoolNotificationSend         Permission = "moderation.school_notification.send"
	PermPlatformAuditRead                        Permission = "platform.audit.read"
	PermPlatformBreakGlassApprove                Permission = "platform.break_glass.approve"
	PermPlatformBreakGlassRequest                Permission = "platform.break_glass.request"
	PermPlatformCatalogWrite                     Permission = "platform.catalog.write"
	PermPlatformCrossTenantRead                  Permission = "platform.cross_tenant.read"
	PermPlatformDataRequestWrite                 Permission = "platform.data_request.write"
	PermPlatformEfficacyStudyExport              Permission = "platform.efficacy_study.export"
	PermPlatformEfficacyStudyManage              Permission = "platform.efficacy_study.manage"
	PermPlatformFeatureFlagWrite                 Permission = "platform.feature_flag.write"
	PermPlatformLearningExperimentManage         Permission = "platform.learning_experiment.manage"
	PermPlatformRoleRead                         Permission = "platform.role.read"
	PermPlatformRoleWrite                        Permission = "platform.role.write"
	PermPlatformSettingsWrite                    Permission = "platform.settings.write"
	PermPlatformSisConnectionManage              Permission = "platform.sis_connection.manage"
	PermPlatformStudentCleanupApprove            Permission = "platform.student_cleanup.approve"
	PermPlatformTenantManage                     Permission = "platform.tenant.manage"
	PermPlatformTenantDataAssign                 Permission = "platform.tenant_data.assign"
	PermPlatformTenantDataClone                  Permission = "platform.tenant_data.clone"
	PermPlatformTrashRead                        Permission = "platform.trash.read"
	PermPlatformTrashRestore                     Permission = "platform.trash.restore"
	PermPlatformUserManage                       Permission = "platform.user.manage"
	PermPlatformUserRoleWrite                    Permission = "platform.user_role.write"
	PermReportExport                             Permission = "report.export"
	PermSchoolPeopleRead                         Permission = "school.people.read"
	PermSchoolSettingsRead                       Permission = "school.settings.read"
	PermSchoolSettingsWrite                      Permission = "school.settings.write"
	PermSocialClubApprove                        Permission = "social.club.approve"
	PermTenantAuditRead                          Permission = "tenant.audit.read"
	PermTenantChatRetentionConfigure             Permission = "tenant.chat_retention.configure"
	PermTenantClassRead                          Permission = "tenant.class.read"
	PermTenantRoleRead                           Permission = "tenant.role.read"
	PermTenantRoleWrite                          Permission = "tenant.role.write"
	PermTenantRolesAssign                        Permission = "tenant.roles.assign"
	PermTenantSchoolRead                         Permission = "tenant.school.read"
	PermTenantSchoolWrite                        Permission = "tenant.school.write"
	PermTenantSettingsRead                       Permission = "tenant.settings.read"
	PermTenantSettingsWrite                      Permission = "tenant.settings.write"
	PermTenantSisImport                          Permission = "tenant.sis.import"
	PermTenantSisRead                            Permission = "tenant.sis.read"
	PermTenantStudentCleanupConfigure            Permission = "tenant.student_cleanup.configure"
	PermTenantStudentCleanupExclude              Permission = "tenant.student_cleanup.exclude"
	PermTenantStudentCleanupRead                 Permission = "tenant.student_cleanup.read"
	PermTenantTrashRead                          Permission = "tenant.trash.read"
	PermTenantTrashRestore                       Permission = "tenant.trash.restore"
	PermTenantUserRead                           Permission = "tenant.user.read"
	PermTenantUserWrite                          Permission = "tenant.user.write"
	PermTenantUserRoleWrite                      Permission = "tenant.user_role.write"
)

var catalog = []Permission{
	PermAnalyticsClassRead,
	PermAnalyticsSchoolRead,
	PermAnalyticsTenantRead,
	PermChatPolicyToggle,
	PermChatSensitiveTermsRead,
	PermChatSensitiveTermsWrite,
	PermChatTenantCategoryToggle,
	PermChatTenantTermsWrite,
	PermCompetitionConfigLeaderboardWrite,
	PermCompetitionConfigMatchmakingWrite,
	PermCompetitionConfigRankingWrite,
	PermCompetitionConfigRewardWrite,
	PermCompetitionConfigSeasonWrite,
	PermCompetitionConfigTournamentTemplateWrite,
	PermContentReleaseRollback,
	PermCurriculumPublish,
	PermCurriculumRead,
	PermCurriculumWrite,
	PermGameAchievementWrite,
	PermGameAnalyticsRead,
	PermGameBossWrite,
	PermGameChallengeWrite,
	PermGameChatAssetsWrite,
	PermGameCollectionWrite,
	PermGameContentPublish,
	PermGameContentReview,
	PermGameContentTranslate,
	PermGameEconomyAdjust,
	PermGameEconomyWrite,
	PermGameEventOperate,
	PermGameEventWrite,
	PermGameItemWrite,
	PermGameLiveopsRead,
	PermGameMinigameWrite,
	PermGameNpcWrite,
	PermGameProgressionWrite,
	PermGameQuestWrite,
	PermGameRewardRuleWrite,
	PermGameSeasonTrackWrite,
	PermGameShopWrite,
	PermGameWorldWrite,
	PermGameWorldEventWrite,
	PermLearningContentActivityReview,
	PermLearningContentActivityWrite,
	PermLearningContentImport,
	PermLearningContentPublish,
	PermLearningContentReEvaluate,
	PermLearningContentRead,
	PermLearningContentReview,
	PermLearningContentTranslate,
	PermLearningContentWrite,
	PermLmsAssignmentRead,
	PermLmsAssignmentWrite,
	PermLmsChallengeWrite,
	PermLmsClassRead,
	PermLmsCourseWrite,
	PermLmsLearningPathWrite,
	PermLmsReportRead,
	PermLmsResultRelease,
	PermLmsScheduleWrite,
	PermLmsSchoolEventWrite,
	PermLmsStudentPasswordReset,
	PermLmsStudentActivityRead,
	PermLmsStudyGroupWrite,
	PermMediaUpload,
	PermModerationCaseAct,
	PermModerationCaseLead,
	PermModerationCaseMessagesRead,
	PermModerationCaseRead,
	PermModerationSanctionRead,
	PermModerationSanctionWrite,
	PermModerationSchoolNotificationAcknowledge,
	PermModerationSchoolNotificationRead,
	PermModerationSchoolNotificationSend,
	PermPlatformAuditRead,
	PermPlatformBreakGlassApprove,
	PermPlatformBreakGlassRequest,
	PermPlatformCatalogWrite,
	PermPlatformCrossTenantRead,
	PermPlatformDataRequestWrite,
	PermPlatformEfficacyStudyExport,
	PermPlatformEfficacyStudyManage,
	PermPlatformFeatureFlagWrite,
	PermPlatformLearningExperimentManage,
	PermPlatformRoleRead,
	PermPlatformRoleWrite,
	PermPlatformSettingsWrite,
	PermPlatformSisConnectionManage,
	PermPlatformStudentCleanupApprove,
	PermPlatformTenantManage,
	PermPlatformTenantDataAssign,
	PermPlatformTenantDataClone,
	PermPlatformTrashRead,
	PermPlatformTrashRestore,
	PermPlatformUserManage,
	PermPlatformUserRoleWrite,
	PermReportExport,
	PermSchoolPeopleRead,
	PermSchoolSettingsRead,
	PermSchoolSettingsWrite,
	PermSocialClubApprove,
	PermTenantAuditRead,
	PermTenantChatRetentionConfigure,
	PermTenantClassRead,
	PermTenantRoleRead,
	PermTenantRoleWrite,
	PermTenantRolesAssign,
	PermTenantSchoolRead,
	PermTenantSchoolWrite,
	PermTenantSettingsRead,
	PermTenantSettingsWrite,
	PermTenantSisImport,
	PermTenantSisRead,
	PermTenantStudentCleanupConfigure,
	PermTenantStudentCleanupExclude,
	PermTenantStudentCleanupRead,
	PermTenantTrashRead,
	PermTenantTrashRestore,
	PermTenantUserRead,
	PermTenantUserWrite,
	PermTenantUserRoleWrite,
}

var catalogSet = func() map[Permission]bool {
	m := make(map[Permission]bool, len(catalog))
	for _, p := range catalog {
		m[p] = true
	}
	return m
}()

// Catalog returns every permission of the catalog.
func Catalog() []Permission { return append([]Permission(nil), catalog...) }

// Known reports whether p is in the catalog.
func (p Permission) Known() bool { return catalogSet[p] }

// platformOnlyPrefixes and platformOnlyCodes may never be held by a tenant role (AGENTS R20, ADR-0027,
// F002 clarification 2026-11-17).
var (
	platformOnlyPrefixes = []string{"game.", "competition.config.", "platform.", "moderation.case.", "moderation.sanction."}
	platformOnlyCodes    = map[Permission]bool{
		PermModerationSchoolNotificationSend: true,
		PermChatSensitiveTermsWrite:          true,
		PermChatPolicyToggle:                 true,
		PermChatTenantTermsWrite:             true,
		PermChatTenantCategoryToggle:         true,
	}
)

// PlatformOnly reports a permission reserved to platform roles (and gated by MFA, F002-AC-18).
func (p Permission) PlatformOnly() bool {
	if platformOnlyCodes[p] {
		return true
	}
	for _, pre := range platformOnlyPrefixes {
		if strings.HasPrefix(string(p), pre) {
			return true
		}
	}
	return false
}
