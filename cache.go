package hoverfly

import (
	"bytes"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/boltdb/bolt"
)

// Cache - cache interface used to store and retrieve request/response payloads or anything else
type Cache interface {
	Set(key, value []byte) error
	Get(key []byte) ([]byte, error)
	GetAllRequests() ([]Payload, error)
	RecordsCount() (int, error)
	DeleteData() error
	GetAllKeys() (map[string]bool, error)
	CloseDB()
}

// NewBoltDBCache - returns new BoltCache instance
func NewBoltDBCache(db *bolt.DB, bucket []byte) *BoltCache {
	return &BoltCache{
		DS:             db,
		RequestsBucket: []byte(bucket),
	}
}

// RequestsBucketName - default name for BoltDB bucket
const RequestsBucketName = "rqbucket"

// BoltCache - container to implement Cache instance with BoltDB backend for storage
type BoltCache struct {
	DS             *bolt.DB
	RequestsBucket []byte
}

// GetDB - returns open BoltDB database with read/write permissions or goes down in flames if
// something bad happends
func GetDB(name string) *bolt.DB {
	log.WithFields(log.Fields{
		"databaseName": name,
	}).Info("Initiating database")
	db, err := bolt.Open(name, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

// CloseDB - closes database
func (c *BoltCache) CloseDB() {
	c.DS.Close()
}

// Set - saves given key and value pair to cache
func (c *BoltCache) Set(key, value []byte) error {
	err := c.DS.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(c.RequestsBucket)
		if err != nil {
			return err
		}
		err = bucket.Put(key, value)
		if err != nil {
			return err
		}
		return nil
	})

	return err
}

// Get - searches for given key in the cache and returns value if found
func (c *BoltCache) Get(key []byte) (value []byte, err error) {

	err = c.DS.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.RequestsBucket)
		if bucket == nil {
			return fmt.Errorf("Bucket %q not found!", c.RequestsBucket)
		}
		// "Byte slices returned from Bolt are only valid during a transaction."
		var buffer bytes.Buffer
		val := bucket.Get(key)

		// If it doesn't exist then it will return nil
		if val == nil {
			return fmt.Errorf("key %q not found \n", key)
		}

		buffer.Write(val)
		value = buffer.Bytes()
		return nil
	})

	return
}

// GetAllRequests - returns all captured requests/responses
func (c *BoltCache) GetAllRequests() (payloads []Payload, err error) {
	err = c.DS.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.RequestsBucket)
		if b == nil {
			// bucket doesn't exist
			return nil
		}
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			pl, err := decodePayload(v)
			if err != nil {
				log.WithFields(log.Fields{
					"error": err.Error(),
					"json":  v,
				}).Warning("Failed to deserialize bytes to payload.")
			} else {
				payloads = append(payloads, *pl)
			}
		}
		return nil
	})
	return
}

// RecordsCount - returns records count
func (c *BoltCache) RecordsCount() (count int, err error) {
	err = c.DS.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.RequestsBucket)
		if b == nil {
			// bucket doesn't exist
			return nil
		}

		count = b.Stats().KeyN

		return nil
	})
	return
}

// DeleteData - deletes bucket with all saved data
func (c *BoltCache) DeleteData() error {
	err := c.DeleteBucket(c.RequestsBucket)
	return err
}

// DeleteBucket - deletes bucket with all saved data
func (c *BoltCache) DeleteBucket(name []byte) (err error) {
	err = c.DS.Update(func(tx *bolt.Tx) error {
		err = tx.DeleteBucket(name)
		if err != nil {
			log.WithFields(log.Fields{
				"error": err.Error(),
				"name":  string(name),
			}).Warning("Failed to delete bucket")

		}
		return err
	})
	return
}

// GetAllKeys - gets all current keys
func (c *BoltCache) GetAllKeys() (keys map[string]bool, err error) {
	err = c.DS.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(c.RequestsBucket)

		keys = make(map[string]bool)

		if b == nil {
			// bucket doesn't exist
			return nil
		}
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			keys[string(k)] = true
		}
		return nil
	})
	return
}
