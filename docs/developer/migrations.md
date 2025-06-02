---
title: "Database Migrations"
sidebar: true
order: 2
---

# Database Migrations

This section covers everything you need to know about managing database migrations in Rapua, including how to run migrations, create new ones, and test them.

## Getting Started with Migrations

Follow these steps to create and run your first migration:

1. **Initialize migrations**:
    ```sh
    ./rapua db init
    ```
    This sets up the internal migration system and creates the necessary tables in the database.

2. **Apply existing migrations:**
    ```sh
    ./rapua db migrate
    ```
    This runs all pending migrations, ensuring your database schema is up-to-date.

3. **Create a new migration:**
    ```sh
     db create_go add_location_groups
    ```
    This generates a new migration file in `internal/migrations` of the form `YYYYMMDDHHMMSS_add_location_groups.go`.

    Edit the file to define the changes you want to make to the database schema:

    ```go
    func init() {
        // Define the LocationGroup model
        type LocationGroup struct {
            ID          string `bun:"id,pk,type:varchar(36)"`
            Name        string `bun:"name,type:varchar(255)"`
            Description string `bun:"description,type:text"`
        }

        Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
            // Up migration
            _, err := db.NewCreateTable().Model((*LocationGroup)(nil)).Exec(ctx)
            return fmt.Errorf("create table location_groups: %w", err)
        }, func(ctx context.Context, db *bun.DB) error {
            // Down migration
            _, err := db.NewDropTable().Model((*LocationGroup)(nil)).Exec(ctx)
            return fmt.Errorf("drop table location_groups: %w", err)
        })
    }
    ```

    **Note:** Rapua uses [Bun](https://bun.uptrace.dev/) for database interactions, so refer to the Bun documentation for more information.

4. **Test the migration:**
    ```sh
    go test ./internal/migrations
    ```

    This ensures the migration can be applied and rolled back without errors.

5. **Apply the new migration:**
    ```sh
    ./rapua db migrate
    ```
    This runs the new migration, updating the database schema to match the latest version.

6. **Verify the migration**:
    Use a database client such as [DB Browser for SQLite](https://sqlitebrowser.org/) to inspect the database schema and verify that the migration was successful.

---

## Reference: CLI Commands

| Command | Description |
|:--------|:------------|
| `./rapua db init` | Initializes the migration system and creates the necessary tables in the database. |
| `./rapua db status` | Displays the current migration status, including the latest version and pending migrations. |
| `./rapua db migrate` | Applies all pending migrations to update the database schema. |
| `./rapua db create_go <name>` | Creates a new migration file in `internal/migrations` with the specified name. |
| `./rapua db rollback` | Rolls back the most recent migration. |
| `go test ./...` | Runs all tests, including migration tests. |

---

## Best Practices

- **Keep models within the init function**: Define models within the `init` function of the migration file to avoid conflicts with other migrations.
    - Alternatively, models may exist outside `init` but must be prefixed with the migration timestamp to ensure uniqueness. This is particularly useful when working with models with circular references.
- **Limit Struct Tags**: Use only `Bun`'s struct tags for defining models. Models must never include any other struct tags such as `json`, `xml`, or `yaml`. This avoids tightly coupling business logic with the database schema. If you're unfamiliar with `Bun`, refer to their [documentation](https://bun.uptrace.dev/).

---

## Testing Migrations

Thorough testing is essential to ensure that database migrations work correctly and do not introduce issues. These guidelines will help you test migrations effectively:

1. **Test in a Staging Environment**: This should go without saying, but always test migrations in a staging environment before applying them in production.
2. **Comprehensive Testing**: Ensure that you include repository and service tests to validate that the application logic works correctly with the new schema. All application tests should pass after applying migrations.

### Built-In Migration Tests

Rapua includes a test to validate that all migrations can be applied and rolled back without errors. This can be run using:

```sh
go test ./internal/migrations
```

- **What It Does**: Ensures migrations can be executed sequentially (both up and down) without errors on an empty, in-memory SQLite database.
- **What It Doesn't Do**: This test does not guarantee that your application logic is compatible with the new schema.

### Ensuring Business Logic Compatibility

To fully verify the new schema:
- **Update Service and Repository Tests**: Modify existing tests or add new ones to validate that services and repositories function as expected with the updated database schema.
- **Run Integration Tests**: Perform integration tests across different layers of your application to confirm that all components work seamlessly with the new schema. These tests can be run using:
    ```sh
    go test ./...
    ```

### Cautions

- **Passing Tests Aren't Everything**: A successful migration test only ensures that the database structure changes as intended. It does not validate that the application will behave correctly with the new schema.
- **Data Loss Risks**: Rolling back migrations can lead to data loss if migrations modify or delete existing columns, tables, or records.
- **Production Precautions**: Always test thoroughly in a staging environment before applying migrations in production. Back up your database as a safeguard against potential data loss or corruption.

By following these steps, you can minimize risks and ensure your migrations are safe and effective.

---

## Migration Grouping

Rapua applies and rolls back migrations in **groups**, providing a clear and consistent approach to managing database changes. Groups are dynamically determined and include all migrations applied after the last successfully completed migration.

### How Grouping Works

1. **Dynamic Grouping**:
   - Migrations are organized into dynamic groups based on the last applied migration.
   - The most recent group always includes all migrations between the last successfully applied migration and the latest migration.

2. **Unified Groups**:
   - Migrations within a group are treated as a single unit, with no sub-groups or partial execution.
   - This ensures that migrations are consistently applied or rolled back together.

For example, if upgrading from `v3.0.0` to `v4.0.0`, all migrations between these versions are treated as a single group. Rolling back from `v4.0.0` reverts all migrations in the `v3.0.0 → v4.0.0` group in a single operation.
