package http

import (
	"fmt"

	"git.amikhalev.com/amikhalev/grinklers/util"
	"github.com/go-resty/resty"
)

var logger = util.Logger.WithField("module", "http")

////////////////////////////////////////
// JSON data types
////////////////////////////////////////

type DeviceRegisterResult struct {
	Data struct {
		DeviceID string `json:"deviceId"`
		Name     string `json:"name"`
		ID       int    `json:"id"`
	} `json:"data"`
	Token string `json:"token"`
}

type DeviceConnectResult struct {
	MqttURL  string `json:"mqttUrl"`
	DeviceID string `json:"deviceId"`
	ClientID string `json:"clientId"`
}

type APIError struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
	Code       int    `json:"code"`
}

func (err *APIError) Error() string {
	return fmt.Sprintf("a sprinklers API error occurred (code %d): %s", err.Code, err.Message)
}

var _ error = (*APIError)(nil)

////////////////////////////////////////
// API
////////////////////////////////////////

func checkAPIError(err interface{}) *APIError {
	apiErr := err.(*APIError)
	if apiErr == nil {
		return &APIError{"Invalid response", 500, util.EC_Internal}
	}
	return apiErr
}

type Config struct {
	ApiURL                  string
	DeviceRegistrationToken string
}

type DeviceData struct {
	DeviceID    string
	DeviceToken string
}

type ConnectData struct {
	DeviceToken string
	*DeviceConnectResult
}

type APIClient struct {
	Config *Config
	Device *DeviceData
	rest   *resty.Client
}

func NewAPIClient(config *Config) *APIClient {
	return &APIClient{
		config,
		nil,
		resty.New().
			SetHostURL(config.ApiURL).
			SetError(&APIError{}),
	}
}

func (a *APIClient) Register() error {
	req := a.rest.NewRequest()
	req.Header.Add("Authorization", "Bearer "+a.Config.DeviceRegistrationToken)
	req.SetResult(&DeviceRegisterResult{})
	res, err := req.Post("/devices/register")
	if err != nil {
		return err
	}
	if res.IsError() {
		return checkAPIError(res.Error())
	}
	result := res.Result().(*DeviceRegisterResult)
	a.Device = &DeviceData{
		DeviceID:    result.Data.DeviceID,
		DeviceToken: result.Token,
	}
	logger.WithField("deviceID", a.Device.DeviceID).Info("device registered")
	return nil
}

func (a *APIClient) Connect() (connectData *ConnectData, err error) {
	if a.Device == nil {
		err = fmt.Errorf("no device data to connect with")
		return
	}
	req := a.rest.NewRequest()
	req.Header.Add("Authorization", "Bearer "+a.Device.DeviceToken)
	req.SetResult(&DeviceConnectResult{})
	res, err := req.Post("/devices/connect")
	if err != nil {
		return
	}
	if res.IsError() {
		err = checkAPIError(res.Error())
		return
	}
	result := res.Result().(*DeviceConnectResult)
	connectData = &ConnectData{DeviceToken: a.Device.DeviceToken, DeviceConnectResult: result}
	logger.WithField("mqttUrl", connectData.MqttURL).Info("device connect success")
	return
}

func (a *APIClient) RegisterAndConnect() (connectData *ConnectData, err error) {
	if a.Device == nil {
		err = a.Register()
		if err != nil {
			return
		}
	}
	connectData, err = a.Connect()
	return
}
