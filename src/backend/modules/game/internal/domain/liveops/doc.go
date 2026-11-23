// Package liveops is the domain of the live-ops events of module game (F077, EVENT_SEASON_SYSTEM §3–§6, §9–§12): the
// event version (audience, schedule, components, event points, milestones, community goal, decoration) and its checks
// (healthy rules, bilingual platform text, decoration budget), the run state machine (scheduled → live → ended →
// archived; suspended and cancelled by an operator), the visibility of a run to a student, the event points and community
// goal contributions under their daily caps, the milestones reached (never lowered by a new version) and the school event
// templates (allowed_params). Pure: no I/O; the clock is passed in.
package liveops
