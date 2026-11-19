// Package curriculum is the domain of the curriculum structure (F021, CURRICULUM_MODEL §2–§6): codes
// (curriculum, LO, competency, topic, version label), the rules of a learning objective, the prerequisite DAG,
// the topic tree, the publish policy of a CurriculumVersion and the LO mapping between versions. A
// CurriculumVersion is the unit of version and release: its subject × grade pairs, topics, LOs and competencies
// change only while it is draft (ADR-0015). Pure Go, no I/O.
package curriculum
