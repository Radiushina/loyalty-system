package user

import "golang.org/x/crypto/bcrypt"

const bcryptCost = bcrypt.DefaultCost

type Hasher struct{}

func NewHasher() *Hasher {
	return &Hasher{}
}

func (p *Hasher) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (p *Hasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
