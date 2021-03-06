package mongo

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/madappgang/identifo/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	"golang.org/x/crypto/bcrypt"
)

const usersCollectionName = "Users"

// NewUserStorage creates and inits MongoDB user storage.
func NewUserStorage(db *DB) (model.UserStorage, error) {
	coll := db.Database.Collection(usersCollectionName)
	us := &UserStorage{coll: coll, timeout: 30 * time.Second}

	userNameIndexOptions := &options.IndexOptions{}
	userNameIndexOptions.SetUnique(true)
	userNameIndexOptions.SetSparse(true)
	userNameIndexOptions.SetCollation(&options.Collation{Locale: "en", Strength: 1})

	userNameIndex := &mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: "username", Value: bsonx.Int32(int32(1))}},
		Options: userNameIndexOptions,
	}

	emailIndexOptions := &options.IndexOptions{}
	emailIndexOptions.SetUnique(true)
	emailIndexOptions.SetSparse(true)

	emailIndex := &mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: "email", Value: bsonx.Int32(int32(1))}},
		Options: emailIndexOptions,
	}

	phoneIndexOptions := &options.IndexOptions{}
	phoneIndexOptions.SetUnique(true)
	phoneIndexOptions.SetSparse(true)

	phoneIndex := &mongo.IndexModel{
		Keys:    bsonx.Doc{{Key: "phone", Value: bsonx.Int32(int32(1))}},
		Options: phoneIndexOptions,
	}

	err := db.EnsureCollectionIndices(usersCollectionName, []mongo.IndexModel{*userNameIndex, *emailIndex, *phoneIndex})
	return us, err
}

// UserStorage implements user storage interface.
type UserStorage struct {
	coll    *mongo.Collection
	timeout time.Duration
}

// NewUser returns pointer to newly created user.
func (us *UserStorage) NewUser() model.User {
	return &User{}
}

// UserByID returns user by its ID.
func (us *UserStorage) UserByID(id string) (model.User, error) {
	hexID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, bson.M{"_id": hexID}).Decode(&u); err != nil {
		return nil, err
	}
	return &User{userData: u}, nil
}

// UserByEmail returns user by their email.
func (us *UserStorage) UserByEmail(email string) (model.User, error) {
	if email == "" {
		return nil, model.ErrorWrongDataFormat
	}
	email = strings.ToLower(email)

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, bson.M{"email": email}).Decode(&u); err != nil {
		return nil, err
	}
	return &User{userData: u}, nil
}

// UserByFederatedID returns user by federated ID.
func (us *UserStorage) UserByFederatedID(provider model.FederatedIdentityProvider, id string) (model.User, error) {
	sid := string(provider) + ":" + id

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, bson.M{"federated_ids": sid}).Decode(&u); err != nil {
		return nil, model.ErrUserNotFound
	}
	//clear password hash
	u.Pswd = ""
	return &User{userData: u}, nil
}

// UserExists checks if user with provided name exists.
func (us *UserStorage) UserExists(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	strictPattern := "^" + name + "$"
	q := bson.D{primitive.E{Key: "username", Value: primitive.Regex{Pattern: strictPattern, Options: "i"}}}

	var u userData
	err := us.coll.FindOne(ctx, q).Decode(&u)
	return err == nil
}

//AttachDeviceToken do nothing here
//TODO: implement device storage
func (us *UserStorage) AttachDeviceToken(id, token string) error {
	//we are not supporting devices for users here
	return model.ErrorNotImplemented
}

//DetachDeviceToken do nothing here yet
//TODO: implement
func (us *UserStorage) DetachDeviceToken(token string) error {
	return model.ErrorNotImplemented
}

//RequestScopes for now returns requested scope
//TODO: implement scope logic
func (us *UserStorage) RequestScopes(userID string, scopes []string) ([]string, error) {
	return scopes, nil
}

// Scopes returns supported scopes, could be static data of database.
func (us *UserStorage) Scopes() []string {
	// we allow all scopes for embedded database, you could implement your own logic in external service.
	return []string{"offline", "user"}
}

// UserByPhone fetches user by phone number.
func (us *UserStorage) UserByPhone(phone string) (model.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, bson.M{"phone": phone}).Decode(&u); err != nil {
		return nil, err
	}
	u.Pswd = ""
	return &User{userData: u}, nil
}

// UserByNamePassword returns user by name and password.
func (us *UserStorage) UserByNamePassword(name, password string) (model.User, error) {
	strictPattern := "^" + strings.ReplaceAll(name, "+", "\\+") + "$"
	q := bson.D{primitive.E{Key: "username", Value: primitive.Regex{Pattern: strictPattern, Options: "i"}}}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, q).Decode(&u); err != nil {
		return nil, model.ErrUserNotFound
	}

	if bcrypt.CompareHashAndPassword([]byte(u.Pswd), []byte(password)) != nil {
		return nil, model.ErrUserNotFound
	}
	//clear password hash
	u.Pswd = ""
	return &User{userData: u}, nil
}

// AddNewUser adds new user to the database.
func (us *UserStorage) AddNewUser(usr model.User, password string) (model.User, error) {
	usr.SetEmail(strings.ToLower(usr.Email()))
	u, ok := usr.(*User)
	if !ok {
		return nil, model.ErrorWrongDataFormat
	}

	u.userData.ID = primitive.NewObjectID()
	if len(password) > 0 {
		u.userData.Pswd = PasswordHash(password)
	}
	u.userData.NumOfLogins = 0

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	if _, err := us.coll.InsertOne(ctx, u.userData); err != nil {
		if isErrDuplication(err) {
			return nil, model.ErrorUserExists
		}
		return nil, err
	}
	return u, nil
}

// AddUserByPhone registers new user with phone number.
func (us *UserStorage) AddUserByPhone(phone, role string) (model.User, error) {
	u := userData{
		ID:          primitive.NewObjectID(),
		Username:    phone,
		Active:      true,
		Phone:       phone,
		AccessRole:  role,
		NumOfLogins: 0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	if _, err := us.coll.InsertOne(ctx, u); err != nil {
		if isErrDuplication(err) {
			return nil, model.ErrorUserExists
		}
		return nil, err
	}
	return &User{userData: u}, nil
}

// AddUserByNameAndPassword registers new user.
func (us *UserStorage) AddUserByNameAndPassword(username, password, role string, isAnonymous bool) (model.User, error) {
	u := userData{
		ID:         primitive.NewObjectID(),
		Active:     true,
		Username:   username,
		AccessRole: role,
		Anonymous:  isAnonymous,
	}
	if model.EmailRegexp.MatchString(u.Username) {
		u.Email = u.Username
	}
	if model.PhoneRegexp.MatchString(u.Username) {
		u.Phone = u.Username
	}
	return us.AddNewUser(&User{userData: u}, password)
}

// AddUserWithFederatedID adds new user with social ID.
func (us *UserStorage) AddUserWithFederatedID(provider model.FederatedIdentityProvider, federatedID, role string) (model.User, error) {
	// If there is no error, it means user already exists.
	if _, err := us.UserByFederatedID(provider, federatedID); err == nil {
		return nil, model.ErrorUserExists
	}

	sid := string(provider) + ":" + federatedID
	u := userData{
		ID:           primitive.NewObjectID(),
		Active:       true,
		Username:     sid,
		AccessRole:   role,
		FederatedIDs: []string{sid},
	}
	return us.AddNewUser(&User{userData: u}, "")
}

// UpdateUser updates user in MongoDB storage.
func (us *UserStorage) UpdateUser(userID string, newUser model.User) (model.User, error) {
	hexID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, err
	}

	newUser.SetEmail(strings.ToLower(newUser.Email()))

	res, ok := newUser.(*User)
	if !ok || res == nil {
		return nil, model.ErrorWrongDataFormat
	}

	// use ID from the request
	res.userData.ID = hexID

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	update := bson.M{"$set": res.userData}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var ud userData
	if err := us.coll.FindOneAndUpdate(ctx, bson.M{"_id": hexID}, update, opts).Decode(&ud); err != nil {
		return nil, err
	}
	return &User{userData: ud}, nil
}

// ResetPassword sets new user's password.
func (us *UserStorage) ResetPassword(id, password string) error {
	hexID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	update := bson.M{"$set": bson.M{"pswd": PasswordHash(password)}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var ud userData
	err = us.coll.FindOneAndUpdate(ctx, bson.M{"_id": hexID}, update, opts).Decode(&ud)
	return err
}

// ResetUsername sets new user's username.
func (us *UserStorage) ResetUsername(id, username string) error {
	hexID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	update := bson.M{"$set": bson.M{"username": username}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var ud userData
	err = us.coll.FindOneAndUpdate(ctx, bson.M{"_id": hexID}, update, opts).Decode(&ud)
	return err
}

// IDByName returns userID by name.
func (us *UserStorage) IDByName(name string) (string, error) {
	strictPattern := "^" + name + "$"
	q := bson.D{primitive.E{Key: "username", Value: primitive.Regex{Pattern: strictPattern, Options: "i"}}}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	var u userData
	if err := us.coll.FindOne(ctx, q).Decode(&u); err != nil {
		return "", model.ErrorNotFound
	}

	user := &User{userData: u}
	if !user.Active() {
		return "", ErrorInactiveUser
	}
	return user.ID(), nil
}

// DeleteUser deletes user by id.
func (us *UserStorage) DeleteUser(id string) error {
	hexID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), us.timeout)
	defer cancel()

	_, err = us.coll.DeleteOne(ctx, bson.M{"_id": hexID})
	return err
}

// FetchUsers fetches users which name satisfies provided filterString.
// Supports pagination.
func (us *UserStorage) FetchUsers(filterString string, skip, limit int) ([]model.User, int, error) {
	q := bson.D{primitive.E{Key: "username", Value: primitive.Regex{Pattern: filterString, Options: "i"}}}

	ctx, cancel := context.WithTimeout(context.Background(), 2*us.timeout)
	defer cancel()

	total, err := us.coll.CountDocuments(ctx, q)
	if err != nil {
		return []model.User{}, 0, err
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{primitive.E{Key: "username", Value: 1}})
	findOptions.SetLimit(int64(limit))
	findOptions.SetSkip(int64(skip))

	curr, err := us.coll.Find(ctx, q, findOptions)
	if err != nil {
		return []model.User{}, 0, err
	}

	usersData := []userData{}
	if err = curr.All(ctx, &usersData); err != nil {
		return []model.User{}, 0, err
	}

	users := make([]model.User, len(usersData))
	for i, ud := range usersData {
		users[i] = &User{userData: ud}
	}
	return users, int(total), err
}

// ImportJSON imports data from JSON.
func (us *UserStorage) ImportJSON(data []byte) error {
	ud := []userData{}
	if err := json.Unmarshal(data, &ud); err != nil {
		return err
	}
	for _, u := range ud {
		pswd := u.Pswd
		u.Pswd = ""
		if _, err := us.AddNewUser(&User{userData: u}, pswd); err != nil {
			return err
		}
	}
	return nil
}

// UpdateLoginMetadata updates user's login metadata.
func (us *UserStorage) UpdateLoginMetadata(userID string) {
	hexID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		log.Printf("Cannot update login metadata of user %s: %s\n", userID, err)
		return
	}

	update := bson.M{
		"$set": bson.M{"latest_login_time": time.Now().Unix()},
		"$inc": bson.M{"num_of_logins": 1},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*us.timeout)
	defer cancel()

	var ud userData
	if err := us.coll.FindOneAndUpdate(ctx, bson.M{"_id": hexID}, update).Decode(&ud); err != nil {
		log.Printf("Cannot update login metadata of user %s: %s\n", userID, err)
	}
}

// Close is a no-op.
func (us *UserStorage) Close() {}

// PasswordHash creates hash with salt for password.
func PasswordHash(pwd string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(hash)
}
