module github.com/mindfries/v2/api

go 1.27

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/mindfries/v2/cli v0.0.0
	github.com/mindfries/v2/contracts v0.0.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace github.com/mindfries/v2/contracts => ../contracts

replace github.com/mindfries/v2/cli => ../cli
