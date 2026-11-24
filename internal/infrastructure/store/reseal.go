package store

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/a1i6ek/vard/internal/platform/crypto/jose"
)

// SealedColumn is a column holding sealed values, each bound (jose.Bind) to
// the table, the column and the row's key, and kept as bytes or as base64
// text.
type SealedColumn struct {
	Table, Column, Key string
	Encoding           *base64.Encoding // nil for bytea
}

// Where is the context a row's value is sealed to.
func (c SealedColumn) Where(key string) jose.Context {
	return jose.Bind(c.Table, c.Column, key)
}

// SealedColumns is every column holding sealed values. A new one is added
// here, and TestEverySealedColumnIsResealed holds it to the model's binding.
var SealedColumns = []SealedColumn{
	{Table: "server_secrets", Column: "value", Key: "name"},
	{Table: "signing_keys", Column: "private_key", Key: "kid"},
	{Table: "social_providers", Column: "client_secret", Key: "id"},
	{Table: "social_providers", Column: "private_key", Key: "id"},
	{Table: "sso_connections", Column: "client_secret", Key: "id"},
	{Table: "sso_connections", Column: "sp_key", Key: "id"},
	{Table: "mail_settings", Column: "password", Key: "id"},
	{Table: "user_factors", Column: "secret", Key: "id", Encoding: base64.StdEncoding},
	{Table: "mfa_factors", Column: "secret", Key: "id", Encoding: base64.StdEncoding},
	{Table: "refresh_tokens", Column: "successor_sealed", Key: "id"},
	{Table: "sso_logins", Column: "saml_response", Key: "relay_hash", Encoding: base64.RawStdEncoding},
}

// resealBatch is how many rows one pass reads.
const resealBatch = 500

// Reseal seals every value of a column again under the sealer's current key,
// in batches in key order, and says how many it changed. A value already
// under the current key is left alone, so a run that stopped is simply run
// again. Each is written back only if it is still the value read, so one
// changed meanwhile keeps its new value.
func (s *Store) Reseal(ctx context.Context, column SealedColumn, sealer *jose.Sealer) (int, error) {
	db := s.db.WithContext(ctx)
	changed, after := 0, ""

	for {
		var rows []struct {
			Key   string
			Value []byte
		}
		query := fmt.Sprintf(`SELECT %[1]s::text AS key, %[2]s AS value FROM %[3]s
			WHERE %[1]s::text > ? AND %[2]s IS NOT NULL ORDER BY %[1]s::text LIMIT ?`, column.Key, valueOf(column), column.Table)
		if err := db.Raw(query, after, resealBatch).Scan(&rows).Error; err != nil {
			return changed, fmt.Errorf("reading %s.%s: %w", column.Table, column.Column, err)
		}
		if len(rows) == 0 {
			return changed, nil
		}

		for _, row := range rows {
			after = row.Key
			sealed := row.Value
			if column.Encoding != nil {
				decoded, err := column.Encoding.DecodeString(string(row.Value))
				if err != nil {
					return changed, fmt.Errorf("%s.%s of %s is not base64: %w", column.Table, column.Column, row.Key, err)
				}
				sealed = decoded
			}
			if len(sealed) == 0 || sealer.Current(sealed) {
				continue
			}

			resealed, err := sealer.Reseal(column.Where(row.Key), sealed)
			if err != nil {
				return changed, fmt.Errorf("%s.%s of %s: %w", column.Table, column.Column, row.Key, err)
			}

			var stored, was any = resealed, row.Value
			if column.Encoding != nil {
				stored, was = column.Encoding.EncodeToString(resealed), string(row.Value)
			}
			result := db.Exec(fmt.Sprintf(`UPDATE %[1]s SET %[2]s = ? WHERE %[3]s::text = ? AND %[2]s = ?`, column.Table, column.Column, column.Key),
				stored, row.Key, was)
			if result.Error != nil {
				return changed, result.Error
			}
			changed += int(result.RowsAffected)
		}
	}
}

// valueOf reads a column as bytes, whatever it is kept as.
func valueOf(column SealedColumn) string {
	if column.Encoding != nil {
		return "convert_to(" + column.Column + ", 'UTF8')"
	}
	return column.Column
}
