package sqlconsole

import (
	"errors"
	"testing"
)

func TestAnalyze(t *testing.T) {
	for _, c := range []struct {
		sql         string
		destructive bool
		err         error
	}{
		{"SELECT 1;", false, nil},
		{"select * from t where a = 1", false, nil},
		{"insert into t values (1)", false, nil},
		{"update t set a=1 where id=2", false, nil},
		{"update t set a=1", true, nil},
		{"DELETE FROM t", true, nil},
		{"delete from t where id=1", false, nil},
		{"drop table t", true, nil},
		{"ALTER TABLE t DROP COLUMN a", true, nil},
		{"truncate t", true, nil},
		{"with x as (delete from t returning id) select * from x", true, nil},
		{"select 'drop table t; delete from x' -- delete", false, nil},
		{"select $$a;b$$", false, nil},
		{"select 1; drop table t", false, ErrMultiple},
		{"/* c */ ; ", false, ErrEmpty},
		{"", false, ErrEmpty},
		{"  commit", false, ErrBlockedVerb},
		{"SET statement_timeout = 0", false, ErrBlockedVerb},
		{"copy t to program 'x'", false, ErrBlockedVerb},
	} {
		info, err := Analyze(c.sql)
		if !errors.Is(err, c.err) || (err == nil && info.Destructive != c.destructive) {
			t.Errorf("%q: destructive=%v err=%v, want %v %v", c.sql, info.Destructive, err, c.destructive, c.err)
		}
	}
}
