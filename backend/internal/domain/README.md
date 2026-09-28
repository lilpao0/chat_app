# Domain layer

`entity/user.go` defines User and application errors; `repository/user_repository.go` defines the create/find contract. `usecase/auth` contains password policy, password/token interfaces and registration/login use cases. The domain must not import presentation, data, Gin, SQL drivers, or JWT libraries. Registration normalizes email and calls the password interface before persistence; repository inputs contain a hash. Login checks credentials before issuing tokens. PublicUser contains no password hash.
