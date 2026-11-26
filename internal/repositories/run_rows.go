package repositories

// runObjectiveRow is one (run, objective) pair, the shared shape the two
// quest-wide logs return.
type runObjectiveRow struct {
	RunCode     string `bun:"run_code"`
	ObjectiveID string `bun:"objective_id"`
}

func groupByRun(rows []runObjectiveRow) map[string][]string {
	byRun := make(map[string][]string, len(rows))
	for _, row := range rows {
		byRun[row.RunCode] = append(byRun[row.RunCode], row.ObjectiveID)
	}
	return byRun
}
