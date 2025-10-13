package db

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/giwty/switch-library-manager/settings"
	"go.uber.org/zap"
	"log"
	"path/filepath"
	"strings"
	"time"
)

const (
	DB_INTERNAL_TABLENAME = "internal-metadata"
)

type PersistentDB struct {
	db *bolt.DB
}

// generateDBName creates a unique DB name based on scan paths
func generateDBName(baseFolder string, scanPaths []string) string {
	// Combine all scan paths into a single string
	pathsString := strings.Join(scanPaths, "|")

	// Generate MD5 hash
	hash := md5.Sum([]byte(pathsString))
	hashString := fmt.Sprintf("%x", hash)

	// Take first 8 characters for readability
	return filepath.Join(baseFolder, fmt.Sprintf("slm_%s.db", hashString[:8]))
}

func NewPersistentDB(baseFolder string) (*PersistentDB, error) {
	// Use default name for backward compatibility
	return NewPersistentDBWithName(baseFolder, "slm.db")
}

func NewPersistentDBWithScanPaths(baseFolder string, scanPaths []string) (*PersistentDB, error) {
	dbPath := generateDBName(baseFolder, scanPaths)
	return NewPersistentDBWithPath(dbPath)
}

func NewPersistentDBWithName(baseFolder string, dbName string) (*PersistentDB, error) {
	dbPath := filepath.Join(baseFolder, dbName)
	return NewPersistentDBWithPath(dbPath)
}

func NewPersistentDBWithPath(dbPath string) (*PersistentDB, error) {
	// Open the db data file.
	// It will be created if it doesn't exist.
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	//set DB version
	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(DB_INTERNAL_TABLENAME))
		if b == nil {
			b, err := tx.CreateBucket([]byte(DB_INTERNAL_TABLENAME))
			if b == nil || err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			err = b.Put([]byte("app_version"), []byte(settings.SLM_VERSION))
			if err != nil {
				zap.S().Warnf("failed to save app_version - %v", err)
				return err
			}
		}
		return nil
	})

	return &PersistentDB{db: db}, nil
}

func (pd *PersistentDB) Close() {
	pd.db.Close()
}

func (pd *PersistentDB) ClearTable(tableName string) error {
	err := pd.db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket([]byte(tableName))
		return err
	})
	return err
}

func (pd *PersistentDB) AddEntry(tableName string, key string, value interface{}) error {
	var err error
	err = pd.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(tableName))
		if b == nil {
			b, err = tx.CreateBucket([]byte(tableName))
			if b == nil || err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
		}
		var bytesBuff bytes.Buffer
		encoder := gob.NewEncoder(&bytesBuff)
		err := encoder.Encode(value)
		if err != nil {
			return err
		}
		err = b.Put([]byte(key), bytesBuff.Bytes())
		return err
	})
	return err
}

func (pd *PersistentDB) GetEntry(tableName string, key string, value interface{}) error {
	err := pd.db.View(func(tx *bolt.Tx) error {

		b := tx.Bucket([]byte(tableName))
		if b == nil {
			return nil
		}
		v := b.Get([]byte(key))
		if v == nil {
			return nil
		}
		d := gob.NewDecoder(bytes.NewReader(v))

		// Decoding the serialized data
		err := d.Decode(value)
		if err != nil {
			return err
		}
		return nil
	})
	return err
}

/*func (pd *PersistentDB) GetEntries() (map[string]*switchfs.ContentMetaAttributes, error) {
	pd.db.View(func(tx *bolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte(METADATA_TABLENAME))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
		}

		return nil
	})
}*/
