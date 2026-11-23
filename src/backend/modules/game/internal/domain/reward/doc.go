// Package reward is the Reward Engine of module game (F074, PROGRESSION_SYSTEM §3, ADR-0016, ADR-0049): the published
// reward rules, the trigger built from a verified server event, and the pure evaluation that turns them into XP awards
// (conditions → base XP → grade multiplier → repeat multiplier → caps, rounded down). No I/O, no clock: the application
// loads the rules, the history and the cap counters and persists the awards. Shared by F075–F077 (grant appliers).
package reward
