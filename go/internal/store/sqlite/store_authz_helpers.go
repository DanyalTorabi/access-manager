package sqlite

// buildInQueryAndArgs appends "AND idColumn IN (?,…)" to baseSQL and returns
// the combined query string and args. baseArgs are the positional args
// corresponding to the placeholders already in baseSQL; one additional arg is
// appended per id in ids.
//
// Accepting idColumn as an explicit parameter (rather than requiring callers to
// append "AND col " to baseSQL themselves) makes the split-point explicit and
// removes the implicit trailing-space convention that the previous API required.
//
// Predicate ordering: any predicates already in baseSQL appear before the
// appended AND idColumn IN (…) clause in the generated query. Callers that
// placed access_mask > 0 at the end of baseSQL will therefore produce queries
// of the form "… AND access_mask > 0 AND col IN (…)". This is intentional and
// semantically equivalent to the reverse order; the decision is documented here
// rather than left as a silent side-effect.
//
// TODO(T05): when MaxLimit is raised near SQLite SQLITE_MAX_VARIABLE_NUMBER,
// add chunking here so the IN clause is split into batches and results are
// merged, rather than exceeding the parameter cap.
func buildInQueryAndArgs(baseSQL, idColumn string, baseArgs []any, ids []string) (string, []any, error) {
	placeholders, err := inPlaceholders(len(ids))
	if err != nil {
		return "", nil, err
	}
	query := baseSQL + ` AND ` + idColumn + ` IN (` + placeholders + `)` // #nosec G202
	args := make([]any, 0, len(baseArgs)+len(ids))
	args = append(args, baseArgs...)
	for _, id := range ids {
		args = append(args, id)
	}
	return query, args, nil
}
