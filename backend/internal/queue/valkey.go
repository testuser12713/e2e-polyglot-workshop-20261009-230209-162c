// Package queue is the Valkey (Redis-protocol) queue client.
//
// The invoice worker consumes JSON messages from the "invoices" list. Messages
// are pushed on the left and consumed from the right, so the list is FIFO.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// InvoicesKey is the Valkey list the worker reads from.
const InvoicesKey = "invoices"

// InvoiceMessage is the payload shape shared with the Python worker.
type InvoiceMessage struct {
	OrderNumber string `json:"order_number"`
}

// Client is a thin Valkey queue client.
type Client struct {
	rdb *redis.Client
}

// New connects to valkeyURL and verifies the connection.
func New(ctx context.Context, valkeyURL string) (*Client, error) {
	opt, err := redis.ParseURL(valkeyURL)
	if err != nil {
		return nil, fmt.Errorf("queue: invalid VALKEY_URL: %w", err)
	}
	rdb := redis.NewClient(opt)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("queue: cannot reach VALKEY_URL: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Ping reports whether the queue is reachable.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// EnqueueInvoice appends one invoice message for orderNumber to the queue.
func (c *Client) EnqueueInvoice(ctx context.Context, orderNumber string) error {
	payload, err := json.Marshal(InvoiceMessage{OrderNumber: orderNumber})
	if err != nil {
		return fmt.Errorf("queue: encode invoice message: %w", err)
	}
	if err := c.rdb.LPush(ctx, InvoicesKey, payload).Err(); err != nil {
		return fmt.Errorf("queue: enqueue invoice: %w", err)
	}
	return nil
}

// Close releases the underlying connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}
