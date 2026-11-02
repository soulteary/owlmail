package mailbox

import "context"

// Inspection exposes aggregate control state, never identities, addresses or secrets.
type Inspection struct {
	Retired          int `json:"retired_mailboxes"`
	Schema           int `json:"schema"`
	Personal         int `json:"personal_mailboxes"`
	Pending          int `json:"pending_mailboxes"`
	PendingMerges    int `json:"pending_merges"`
	PendingRetention int `json:"pending_retention"`
}

func (s *Store) Inspect(ctx context.Context) (Inspection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := Inspection{}
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&result.Schema); err != nil {
		return result, err
	}
	for _, query := range []struct {
		sql  string
		dest *int
	}{
		{`SELECT count(*) FROM mailbox WHERE state='personal'`, &result.Personal},
		{`SELECT count(*) FROM mailbox WHERE state='pending'`, &result.Pending},
		{`SELECT count(*) FROM merge_journal WHERE state='pending'`, &result.PendingMerges},
	} {
		if err := s.db.QueryRowContext(ctx, query.sql).Scan(query.dest); err != nil {
			return result, err
		}
	}
	if result.Schema >= 2 {
		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM mailbox WHERE retired_at IS NOT NULL`).Scan(&result.Retired); err != nil {
			return result, err
		}
		result.Pending -= result.Retired

		if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM retention_journal WHERE state='pending'`).Scan(&result.PendingRetention); err != nil {
			return result, err
		}
	}
	return result, nil
}
