package edge_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
)

func newPool(t *testing.T) *pgxpool.Pool { return dbtest.NewPool(t) }
