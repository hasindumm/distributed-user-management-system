package ports

type Subscription interface {
	Unsubscribe() error
}
