package database

import (
	"context"
	"database/sql"
	"fmt"
	"inventory-tui/internal/domain/entity"
)

// SQLiteCustomGroupRepository implementa la interfaz CustomGroupRepository usando SQLite.
type SQLiteCustomGroupRepository struct {
	db *sql.DB
}

// NewSQLiteCustomGroupRepository crea una nueva instancia del repositorio de grupos personalizados.
func NewSQLiteCustomGroupRepository(db *sql.DB) *SQLiteCustomGroupRepository {
	return &SQLiteCustomGroupRepository{db: db}
}

// CreateGroup crea un nuevo grupo con los IDs de productos especificados.
func (r *SQLiteCustomGroupRepository) CreateGroup(ctx context.Context, groupName string, productIDs []int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	// Insertar el grupo
	res, err := tx.ExecContext(ctx, "INSERT INTO custom_groups (group_name) VALUES (?)", groupName)
	if err != nil {
		return fmt.Errorf("error al crear grupo: %w", err)
	}

	groupID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("error al obtener ID del grupo: %w", err)
	}

	// Insertar las relaciones con productos
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO custom_group_products (group_id, product_id) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("error al preparar statement: %w", err)
	}
	defer stmt.Close()

	for _, productID := range productIDs {
		if _, err := stmt.ExecContext(ctx, groupID, productID); err != nil {
			return fmt.Errorf("error al asociar producto %d al grupo: %w", productID, err)
		}
	}

	return tx.Commit()
}

// GetGroupByName busca un grupo por su nombre.
func (r *SQLiteCustomGroupRepository) GetGroupByName(ctx context.Context, groupName string) (*entity.CustomGroup, error) {
	var group entity.CustomGroup
	err := r.db.QueryRowContext(ctx, "SELECT id, group_name FROM custom_groups WHERE group_name = ?", groupName).
		Scan(&group.ID, &group.GroupName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Obtener los productos del grupo
	rows, err := r.db.QueryContext(ctx, "SELECT product_id FROM custom_group_products WHERE group_id = ?", group.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var productID int
		if err := rows.Scan(&productID); err != nil {
			return nil, err
		}
		group.ProductIDs = append(group.ProductIDs, productID)
	}

	return &group, nil
}

// GetAllGroups devuelve todos los grupos existentes.
func (r *SQLiteCustomGroupRepository) GetAllGroups(ctx context.Context) ([]entity.CustomGroup, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, group_name FROM custom_groups")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []entity.CustomGroup
	for rows.Next() {
		var group entity.CustomGroup
		if err := rows.Scan(&group.ID, &group.GroupName); err != nil {
			return nil, err
		}

		// Obtener los productos de cada grupo
		productRows, err := r.db.QueryContext(ctx, "SELECT product_id FROM custom_group_products WHERE group_id = ?", group.ID)
		if err != nil {
			return nil, err
		}
		for productRows.Next() {
			var productID int
			if err := productRows.Scan(&productID); err != nil {
				productRows.Close()
				return nil, err
			}
			group.ProductIDs = append(group.ProductIDs, productID)
		}
		productRows.Close()

		groups = append(groups, group)
	}

	return groups, nil
}

// DeleteGroup elimina un grupo por su ID.
func (r *SQLiteCustomGroupRepository) DeleteGroup(ctx context.Context, groupID int) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM custom_groups WHERE id = ?", groupID)
	return err
}

// IsProductInGroup verifica si un producto pertenece a un grupo específico.
func (r *SQLiteCustomGroupRepository) IsProductInGroup(ctx context.Context, groupID int, productID int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM custom_group_products WHERE group_id = ? AND product_id = ?",
		groupID, productID).Scan(&count)
	return count > 0, err
}

// GetGroupsForProduct devuelve todos los grupos que contienen un producto.
func (r *SQLiteCustomGroupRepository) GetGroupsForProduct(ctx context.Context, productID int) ([]entity.CustomGroup, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT cg.id, cg.group_name FROM custom_groups cg JOIN custom_group_products cgp ON cg.id = cgp.group_id WHERE cgp.product_id = ?",
		productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []entity.CustomGroup
	for rows.Next() {
		var group entity.CustomGroup
		if err := rows.Scan(&group.ID, &group.GroupName); err != nil {
			return nil, err
		}

		// Obtener todos los productos del grupo
		productRows, err := r.db.QueryContext(ctx, "SELECT product_id FROM custom_group_products WHERE group_id = ?", group.ID)
		if err != nil {
			return nil, err
		}
		for productRows.Next() {
			var pid int
			if err := productRows.Scan(&pid); err != nil {
				productRows.Close()
				return nil, err
			}
			group.ProductIDs = append(group.ProductIDs, pid)
		}
		productRows.Close()

		groups = append(groups, group)
	}

	return groups, nil
}
