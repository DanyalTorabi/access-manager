package sqlite

// buildInQueryAndArgs appends an IN (?,…) clause to baseSQL and returns the
// combined query string and args. baseArgs are the positional args that
// correspond to the placeholders already in baseSQL; one additional arg is
// appended for each id in ids.
//
// It delegates placeholder generation to inPlaceholders and returns an error
// if ids is empty (which inPlaceholders rejects).
//
// TODO(T05): when MaxLimit is raised near SQLite SQLITE_MAX_VARIABLE_NUMBER,
// add chunking here so the IN clause is split into batches and results are
// merged, rather than exceeding the parameter cap.
func buildInQueryAndArgs(baseSQL string, baseArgs []any, ids []string) (string, []any, error) {
	placeholders, err := inPlaceholders(len(ids))
	if err != nil {
		return "", nil, err
	}
	query := baseSQL + `IN (` + placeholders + `)` // #nosec G202
	args := make([]any, 0, len(baseArgs)+len(ids))
	args = append(args, baseArgs...)
	for _, id := range ids {
		args = append(args, id)
	}
	return query, args, nil
}
