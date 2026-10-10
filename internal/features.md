# Feature boundaries

Future packages: auth, customer, request, quotation, job, material, procurement,
delivery, payment. Each has handler.go, service.go, repository.go (interfaces),
domain.go and focused tests. Create code only when its phase is authorized.
PostgreSQL implementations are in internal/repository/postgres. Services depend
on repository interfaces and do not import pgx or run SQL.
