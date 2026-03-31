package userclient

import "sync"

type cache struct {
	mu    sync.RWMutex
	users map[string]UserDTO
}

func newCache() *cache {
	return &cache{
		users: make(map[string]UserDTO),
	}
}

func (c *cache) get(id string) (UserDTO, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	u, ok := c.users[id]
	return u, ok
}

func (c *cache) set(user UserDTO) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[user.UserID] = user
}

func (c *cache) delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.users, id)
}

func (c *cache) setAll(users []UserDTO) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users = make(map[string]UserDTO, len(users))
	for _, u := range users {
		c.users[u.UserID] = u
	}
}

func (c *cache) getByEmail(email string) (UserDTO, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, u := range c.users {
		if u.Email == email {
			return u, true
		}
	}
	return UserDTO{}, false
}

func (c *cache) list(status *string) []UserDTO {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]UserDTO, 0, len(c.users))
	for _, u := range c.users {
		if status == nil || u.Status == *status {
			result = append(result, u)
		}
	}
	return result
}
