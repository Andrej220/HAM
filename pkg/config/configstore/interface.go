package configstore

type ConfigStore interface {
	Load(out any)  error
	Save(data any) error
}