package account

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
	dto "github.com/na50r/wombo-combo-go-be/dto"
	u "github.com/na50r/wombo-combo-go-be/utility"
	st "github.com/na50r/wombo-combo-go-be/storage"
)

type AccountService struct {
	store st.Storage
}

func NewAccountService(store st.Storage) *AccountService {
	return &AccountService{store: store}
}

// handleGetAccount godoc
// @Summary Get an account
// @Description Get an account
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} dto.AccountDTO
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /account/{username} [get]
func (s *AccountService) handleGetAccount(w http.ResponseWriter, r *http.Request) error {
	username, err := u.GetUsername(r)
	if err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	img, err := s.store.GetImage(acc.ImageName)
	if err != nil {
		return err
	}

	resp := new(dto.AccountDTO)
	resp.Username = acc.Username
	resp.Image = img
	resp.ImageName = acc.ImageName
	resp.CreatedAt = acc.CreatedAt
	resp.Wins = acc.Wins
	resp.Losses = acc.Losses
	resp.Status = acc.Status
	return u.WriteJSON(w, http.StatusOK, resp)
}

// HandleGetImages godoc
// @Summary Get all potential profile pictures
// @Description Get all potential profile pictures
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param username path string true "Username"
// @Success 200 {object} dto.ImagesResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /account/{username}/images [get]
func (s *AccountService) HandleGetImages(w http.ResponseWriter, r *http.Request) error {
	images, err := s.store.GetImages()
	if err != nil {
		return err
	}
	names := make([]string, 0, len(images))
	for _, image := range images {
		names = append(names, image.Name)
	}
	resp := dto.ImagesResponse{Names: names}
	return u.WriteJSON(w, http.StatusOK, resp)
}

func (s *AccountService) HandleAccount(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		return s.handleGetAccount(w, r)
	case http.MethodPut:
		return s.handleEditAccount(w, r)
	default:
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
}

// handleEditAccount godoc
// @Summary Edit an account
// @Description Edit an account
// @Tags account
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param account body dto.EditAccountRequest true "Account to edit"
// @Param username path string true "Username"
// @Success 200 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /account/{username} [put]
func (s *AccountService) handleEditAccount(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	req := new(dto.EditAccountRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}
	username, err := u.GetUsername(r)
	if err != nil {
		return err
	}
	acc, err := s.store.GetAccountByUsername(username)
	if err != nil {
		return err
	}
	var msg string
	if req.Type == "PASSWORD" {
		if err := bcrypt.CompareHashAndPassword([]byte(acc.Password), []byte(req.OldPassword)); err != nil {
			return fmt.Errorf("Incorrect password, please try again")
		}
		if err := u.PasswordValid(req.NewPassword); err != nil {
			return err
		}
		encpw, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		acc.Password = string(encpw)
		msg = "Password changed"
	}
	if req.Type == "USERNAME" {
		acc.Username = req.Username
		msg = "Username changed"
	}
	if req.Type == "IMAGE" {
		acc.ImageName = req.ImageName
		msg = "Image changed"
	}
	if err := s.store.UpdateAccount(acc); err != nil {
		return err
	}
	return u.WriteJSON(w, http.StatusOK, dto.GenericResponse{Message: msg})
}

// HandleRegister godoc
// @Summary Register an account
// @Description Register an account
// @Tags account
// @Accept json
// @Produce json
// @Param account body dto.RegisterRequest true "Account to register"
// @Success 201 {object} dto.GenericResponse
// @Failure 400 {object} dto.APIError
// @Failure 405 {object} dto.APIError
// @Router /accounts [post]
func (s *AccountService) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		err := u.WriteJSON(w, http.StatusMethodNotAllowed, dto.APIError{Error: "Method not allowed"})
		return err
	}
	req := new(dto.RegisterRequest)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		return err
	}

	if err := u.PasswordValid(req.Password); err != nil {
		return err
	}

	acc, err := st.NewAccount(req.Username, req.Password)
	if err != nil {
		return err
	}
	imageName := s.store.NewImageForUsername(acc.Username)
	acc.ImageName = imageName

	if err := s.store.CreateAccount(acc); err != nil {
		log.Println(err)
		return u.WriteJSON(w, http.StatusConflict, dto.APIError{Error: "Username taken, choose another one"})
	}
	return u.WriteJSON(w, http.StatusCreated, dto.GenericResponse{Message: "Account created"})
}

