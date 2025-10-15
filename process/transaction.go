package process

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// FileOperation represents a single file system operation
type FileOperation struct {
	Type     OperationType `json:"type"`
	From     string        `json:"from"`
	To       string        `json:"to"`
	Original string        `json:"original,omitempty"` // For rollback of renames
	GameName string        `json:"game_name"`
}

// OperationType defines the type of operation
type OperationType string

const (
	OpMove   OperationType = "move"
	OpCreate OperationType = "create"
	OpDelete OperationType = "delete"
)

// Transaction tracks file operations for rollback capability
type Transaction struct {
	operations []FileOperation
	committed  bool
	logger     *zap.SugaredLogger
}

// NewTransaction creates a new transaction
func NewTransaction() *Transaction {
	return &Transaction{
		operations: make([]FileOperation, 0),
		committed:  false,
		logger:     zap.S(),
	}
}

// AddMoveOperation adds a file move operation to the transaction
func (t *Transaction) AddMoveOperation(from, to, gameName string) {
	t.operations = append(t.operations, FileOperation{
		Type:     OpMove,
		From:     from,
		To:       to,
		GameName: gameName,
	})
}

// AddCreateOperation adds a folder creation operation to the transaction
func (t *Transaction) AddCreateOperation(path, gameName string) {
	t.operations = append(t.operations, FileOperation{
		Type:     OpCreate,
		To:       path,
		GameName: gameName,
	})
}

// AddDeleteOperation adds a file deletion operation to the transaction
func (t *Transaction) AddDeleteOperation(path, gameName string) {
	t.operations = append(t.operations, FileOperation{
		Type:     OpDelete,
		From:     path,
		GameName: gameName,
	})
}

// Execute executes all operations in the transaction
func (t *Transaction) Execute() error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}

	for i, op := range t.operations {
		if err := t.executeOperation(op, i); err != nil {
			// Operation failed, rollback all previous operations
			t.logger.Errorf("Operation failed at step %d: %v", i, err)
			t.logger.Info("Rolling back transaction...")
			if rollbackErr := t.rollback(i - 1); rollbackErr != nil {
				t.logger.Errorf("Rollback failed: %v", rollbackErr)
				return fmt.Errorf("operation failed: %v, rollback also failed: %v", err, rollbackErr)
			}
			return fmt.Errorf("operation failed and transaction rolled back: %v", err)
		}
	}

	t.committed = true
	t.logger.Infof("Transaction committed successfully with %d operations", len(t.operations))
	return nil
}

// executeOperation executes a single operation
func (t *Transaction) executeOperation(op FileOperation, index int) error {
	switch op.Type {
	case OpMove:
		return t.executeMoveOperation(op, index)
	case OpCreate:
		return t.executeCreateOperation(op, index)
	case OpDelete:
		return t.executeDeleteOperation(op, index)
	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// executeMoveOperation executes a file move operation
func (t *Transaction) executeMoveOperation(op FileOperation, index int) error {
	if op.From == op.To {
		return nil
	}

	// Check if source file exists
	if _, err := os.Stat(op.From); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", op.From)
	}

	// Check if destination directory exists, create if needed
	destDir := filepath.Dir(op.To)
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %v", destDir, err)
		}
	}

	// Check if destination file already exists
	if _, err := os.Stat(op.To); !os.IsNotExist(err) {
		return fmt.Errorf("destination file already exists: %s", op.To)
	}

	// Perform the move
	if err := os.Rename(op.From, op.To); err != nil {
		return fmt.Errorf("failed to move file from %s to %s: %v", op.From, op.To, err)
	}

	// Update the operation with actual result for rollback
	t.operations[index].Original = op.From

	t.logger.Debugf("Moved file: %s -> %s", op.From, op.To)
	return nil
}

// executeCreateOperation executes a folder creation operation
func (t *Transaction) executeCreateOperation(op FileOperation, index int) error {
	// Check if folder already exists
	if _, err := os.Stat(op.To); !os.IsNotExist(err) {
		// Folder exists, nothing to do
		return nil
	}

	// Create the folder
	if err := os.MkdirAll(op.To, 0755); err != nil {
		return fmt.Errorf("failed to create folder %s: %v", op.To, err)
	}

	t.logger.Debugf("Created folder: %s", op.To)
	return nil
}

// executeDeleteOperation executes a file deletion operation
func (t *Transaction) executeDeleteOperation(op FileOperation, index int) error {
	// Check if file exists
	if _, err := os.Stat(op.From); os.IsNotExist(err) {
		// File doesn't exist, nothing to do
		return nil
	}

	// Delete the file
	if err := os.Remove(op.From); err != nil {
		return fmt.Errorf("failed to delete file %s: %v", op.From, err)
	}

	t.logger.Debugf("Deleted file: %s", op.From)
	return nil
}

// rollback rolls back operations up to the specified index
func (t *Transaction) rollback(lastSuccessfulIndex int) error {
	var rollbackErrors []error

	// Rollback operations in reverse order
	for i := lastSuccessfulIndex; i >= 0; i-- {
		op := t.operations[i]
		if err := t.rollbackOperation(op); err != nil {
			rollbackErrors = append(rollbackErrors, err)
			t.logger.Errorf("Failed to rollback operation %d: %v", i, err)
		}
	}

	if len(rollbackErrors) > 0 {
		return fmt.Errorf("rollback completed with %d errors: %v", len(rollbackErrors), rollbackErrors)
	}

	t.logger.Info("Transaction rollback completed successfully")
	return nil
}

// rollbackOperation rolls back a single operation
func (t *Transaction) rollbackOperation(op FileOperation) error {
	switch op.Type {
	case OpMove:
		return t.rollbackMoveOperation(op)
	case OpCreate:
		return t.rollbackCreateOperation(op)
	case OpDelete:
		return t.rollbackDeleteOperation(op)
	default:
		return fmt.Errorf("unknown operation type for rollback: %s", op.Type)
	}
}

// rollbackMoveOperation rolls back a file move operation
func (t *Transaction) rollbackMoveOperation(op FileOperation) error {
	// Move the file back to its original location
	if op.Original != "" && op.Original != op.To {
		// Check if the moved file still exists at destination
		if _, err := os.Stat(op.To); os.IsNotExist(err) {
			t.logger.Warnf("File %s no longer exists at destination during rollback", op.To)
			return nil
		}

		// Move it back
		if err := os.Rename(op.To, op.Original); err != nil {
			return fmt.Errorf("failed to rollback move from %s to %s: %v", op.To, op.Original, err)
		}

		t.logger.Debugf("Rolled back move: %s -> %s", op.To, op.Original)
	}
	return nil
}

// rollbackCreateOperation rolls back a folder creation operation
func (t *Transaction) rollbackCreateOperation(op FileOperation) error {
	// Only remove the folder if it's empty (to avoid removing user data)
	if entries, err := os.ReadDir(op.To); err == nil && len(entries) == 0 {
		if err := os.Remove(op.To); err != nil {
			return fmt.Errorf("failed to rollback folder creation %s: %v", op.To, err)
		}
		t.logger.Debugf("Rolled back folder creation: %s", op.To)
	} else {
		t.logger.Warnf("Folder %s is not empty, skipping rollback removal", op.To)
	}
	return nil
}

// rollbackDeleteOperation rolls back a file deletion operation
func (t *Transaction) rollbackDeleteOperation(op FileOperation) error {
	// Cannot restore a deleted file, this is a limitation
	t.logger.Warnf("Cannot rollback file deletion: %s (file permanently deleted)", op.From)
	return nil
}

// GetOperations returns a copy of all operations in the transaction
func (t *Transaction) GetOperations() []FileOperation {
	operations := make([]FileOperation, len(t.operations))
	copy(operations, t.operations)
	return operations
}

// IsCommitted returns whether the transaction has been committed
func (t *Transaction) IsCommitted() bool {
	return t.committed
}

// Size returns the number of operations in the transaction
func (t *Transaction) Size() int {
	return len(t.operations)
}