package rabbitmq

import (
	"encoding/json"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client RabbitMQ 客户端（生产者 + 消费者）
type Client struct {
	url    string
	conn   *amqp.Connection
	ch     *amqp.Channel
	mu     sync.Mutex
	closed bool
}

// New 创建 RabbitMQ 客户端
func New(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Client{url: url, conn: conn, ch: ch}, nil
}

// Publish 向队列发布 JSON 消息
func (c *Client) Publish(queue string, body interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return amqp.ErrClosed
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	q, err := c.ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	return c.ch.Publish("", q.Name, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         data,
		DeliveryMode: amqp.Persistent,
	})
}

// Consume 消费队列，handler 返回 nil 表示 ACK，非 nil 表示 NACK/重试
func (c *Client) Consume(queue string, handler func([]byte) error) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return amqp.ErrClosed
	}
	q, err := c.ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	deliveries, err := c.ch.Consume(q.Name, "", false, false, false, false, nil)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	for d := range deliveries {
		if err := handler(d.Body); err != nil {
			_ = d.Nack(false, true)
			continue
		}
		_ = d.Ack(false)
	}
	return nil
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Reconnect 重连（简单实现：关闭后新建）
func (c *Client) Reconnect() error {
	c.mu.Lock()
	c.closed = true
	if c.ch != nil {
		_ = c.ch.Close()
		c.ch = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	time.Sleep(time.Second)
	newClient, err := New(c.url)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = newClient.conn
	c.ch = newClient.ch
	c.closed = false
	c.mu.Unlock()
	return nil
}

// EnsureQueue 声明队列（幂等）
func (c *Client) EnsureQueue(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.ch.QueueDeclare(name, true, false, false, false, nil)
	return err
}
