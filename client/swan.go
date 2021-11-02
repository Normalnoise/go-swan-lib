package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/filswan/go-swan-lib/constants"
	"github.com/filswan/go-swan-lib/logs"
	"github.com/filswan/go-swan-lib/model"
	"github.com/filswan/go-swan-lib/utils"
)

const GET_OFFLINEDEAL_LIMIT_DEFAULT = 50
const RESPONSE_STATUS_SUCCESS = "SUCCESS"

type TokenAccessInfo struct {
	ApiKey      string `json:"apikey"`
	AccessToken string `json:"access_token"`
}

type SwanClient struct {
	ApiUrl   string
	JwtToken string
}

type GetOfflineDealResponse struct {
	Data   GetOfflineDealData `json:"data"`
	Status string             `json:"status"`
}

type GetOfflineDealData struct {
	Deal []model.OfflineDeal `json:"deal"`
}

type UpdateOfflineDealResponse struct {
	Data   UpdateOfflineDealData `json:"data"`
	Status string                `json:"status"`
}

type UpdateOfflineDealData struct {
	Deal    model.OfflineDeal `json:"deal"`
	Message string            `json:"message"`
}

func (swanClient *SwanClient) SwanGetJwtToken(apiKey, accessToken string) error {
	data := TokenAccessInfo{
		ApiKey:      apiKey,
		AccessToken: accessToken,
	}

	if len(swanClient.ApiUrl) == 0 {
		err := fmt.Errorf("api url is required")
		logs.GetLogger().Error(err)
		return err
	}

	if len(data.ApiKey) == 0 {
		err := fmt.Errorf("api key is required")
		logs.GetLogger().Error(err)
		return err
	}

	if len(data.AccessToken) == 0 {
		err := fmt.Errorf("acess token is required")
		logs.GetLogger().Error(err)
		return err
	}

	apiUrl := swanClient.ApiUrl + "/user/api_keys/jwt"

	response := HttpPostNoToken(apiUrl, data)

	if len(response) == 0 {
		err := fmt.Errorf("no response from swan platform:%s", apiUrl)
		logs.GetLogger().Error(err)
		return err
	}

	if strings.Contains(response, "fail") {
		message := utils.GetFieldStrFromJson(response, "message")
		status := utils.GetFieldStrFromJson(response, "status")
		err := fmt.Errorf("status:%s,message:%s", status, message)
		logs.GetLogger().Error(err)

		if message == "api_key Not found" {
			logs.GetLogger().Error("please check api_key in ~/.swan/provider/config.toml")
		}

		if message == "please provide a valid api token" {
			logs.GetLogger().Error("Please check access_token in ~/.swan/provider/config.toml")
		}

		logs.GetLogger().Info("for more information about how to config, please check https://docs.filswan.com/run-swan-provider/config-swan-provider")

		return err
	}

	jwtToken := utils.GetFieldMapFromJson(response, "data")
	if jwtToken == nil {
		err := fmt.Errorf("error: fail to connect to swan api")
		logs.GetLogger().Error(err)
		return err
	}

	swanClient.JwtToken = jwtToken["jwt"].(string)

	return nil
}

func SwanGetClient(apiUrl, apiKey, accessToken, jwtToken string) (*SwanClient, error) {
	if len(apiUrl) == 0 {
		err := fmt.Errorf("api url is required")
		logs.GetLogger().Error(err)
		return nil, err
	}

	if jwtToken == constants.EMPTY_STRING {
		swanClient, err := SwanGetClientByKey(apiUrl, apiKey, accessToken)
		return swanClient, err
	}

	swanClient := &SwanClient{
		ApiUrl:   apiUrl,
		JwtToken: jwtToken,
	}

	return swanClient, nil
}

func SwanGetClientByKey(apiUrl, apiKey, accessToken string) (*SwanClient, error) {
	if len(apiUrl) == 0 {
		err := fmt.Errorf("api url is required")
		logs.GetLogger().Error(err)
		return nil, err
	}

	if len(apiKey) == 0 {
		err := fmt.Errorf("api key is required")
		logs.GetLogger().Error(err)
		return nil, err
	}

	if len(accessToken) == 0 {
		err := fmt.Errorf("access token is required")
		logs.GetLogger().Error(err)
		return nil, err
	}

	swanClient := &SwanClient{
		ApiUrl: apiUrl,
	}

	var err error
	for i := 0; i < 3; i++ {
		err = swanClient.SwanGetJwtToken(apiKey, accessToken)
		if err != nil {
			logs.GetLogger().Error(err)
		} else {
			break
		}
	}

	if err != nil {
		err = fmt.Errorf("failed to connect to swan platform after trying 3 times")
		logs.GetLogger().Error(err)
		return nil, err
	}

	return swanClient, err
}

func (swanClient *SwanClient) SwanGetOfflineDeals(minerFid, status string, limit ...string) []model.OfflineDeal {
	rowLimit := strconv.Itoa(GET_OFFLINEDEAL_LIMIT_DEFAULT)
	if len(limit) > 0 {
		rowLimit = limit[0]
	}

	urlStr := swanClient.ApiUrl + "/offline_deals/" + minerFid + "?deal_status=" + status + "&limit=" + rowLimit + "&offset=0"
	response := HttpGet(urlStr, swanClient.JwtToken, "")
	getOfflineDealResponse := GetOfflineDealResponse{}
	err := json.Unmarshal([]byte(response), &getOfflineDealResponse)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil
	}

	if strings.ToUpper(getOfflineDealResponse.Status) != RESPONSE_STATUS_SUCCESS {
		logs.GetLogger().Error("Get offline deal with status ", status, " failed")
		return nil
	}

	return getOfflineDealResponse.Data.Deal
}

func (swanClient *SwanClient) SwanUpdateOfflineDealStatus(dealId int, status string, statusInfo ...string) bool {
	if len(status) == 0 {
		logs.GetLogger().Error("Please provide status")
		return false
	}

	apiUrl := swanClient.ApiUrl + "/my_miner/deals/" + strconv.Itoa(dealId)

	params := url.Values{}
	params.Add("status", status)

	if len(statusInfo) > 0 {
		params.Add("note", statusInfo[0])
	}

	if len(statusInfo) > 1 {
		params.Add("file_path", statusInfo[1])
	}

	if len(statusInfo) > 2 {
		params.Add("file_size", statusInfo[2])
	}

	response := HttpPut(apiUrl, swanClient.JwtToken, strings.NewReader(params.Encode()))

	updateOfflineDealResponse := &UpdateOfflineDealResponse{}
	err := json.Unmarshal([]byte(response), updateOfflineDealResponse)
	if err != nil {
		logs.GetLogger().Error(err)
		return false
	}

	if strings.ToUpper(updateOfflineDealResponse.Status) != RESPONSE_STATUS_SUCCESS {
		logs.GetLogger().Error("Update offline deal with status ", status, " failed.", updateOfflineDealResponse.Data.Message)
		return false
	}

	return true
}

type SwanCreateTaskResponse struct {
	Data    SwanCreateTaskResponseData `json:"data"`
	Status  string                     `json:"status"`
	Message string                     `json:"message"`
}

type SwanCreateTaskResponseData struct {
	Filename string `json:"filename"`
	Uuid     string `json:"uuid"`
}

func (swanClient *SwanClient) SwanCreateTask(task model.Task, csvFilePath string) (*SwanCreateTaskResponse, error) {
	apiUrl := swanClient.ApiUrl + "/tasks"

	params := map[string]string{}
	params["task_name"] = task.TaskName
	params["curated_dataset"] = task.CuratedDataset
	params["description"] = task.Description
	params["is_public"] = strconv.Itoa(*task.IsPublic)

	params["type"] = *task.Type

	if task.MinerFid != nil {
		params["miner_id"] = *task.MinerFid
	}
	params["fast_retrieval"] = strconv.FormatBool(task.FastRetrievalBool)
	params["bid_mode"] = strconv.Itoa(*task.BidMode)
	params["max_price"] = (*task.MaxPrice).String()
	params["expire_days"] = strconv.Itoa(*task.ExpireDays)

	response, err := HttpPostFile(apiUrl, swanClient.JwtToken, params, "file", csvFilePath)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	swanCreateTaskResponse := &SwanCreateTaskResponse{}
	err = json.Unmarshal([]byte(response), swanCreateTaskResponse)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if swanCreateTaskResponse.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s,%s", swanCreateTaskResponse.Status, swanCreateTaskResponse.Message)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return swanCreateTaskResponse, nil
}

type GetTaskByUuidResult struct {
	Data   GetTaskResultData `json:"data"`
	Status string            `json:"status"`
}

type GetTaskByUuidResultData struct {
	Task           model.Task `json:"task"`
	TotalItems     int        `json:"total_items"`
	TotalTaskCount int        `json:"total_task_count"`
}

func (swanClient *SwanClient) SwanGetTaskByUuid(uuid string) (*GetTaskByUuidResult, error) {
	apiUrl := swanClient.ApiUrl + "/tasks/" + uuid
	//logs.GetLogger().Info("Getting My swan tasks info")
	response := HttpGet(apiUrl, swanClient.JwtToken, "")

	if response == "" {
		err := errors.New("failed to get tasks from swan")
		logs.GetLogger().Error(err)
		return nil, err
	}

	//logs.GetLogger().Info(response)

	getTaskByUuidResult := &GetTaskByUuidResult{}
	err := json.Unmarshal([]byte(response), getTaskByUuidResult)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if getTaskByUuidResult.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s", getTaskByUuidResult.Status)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return getTaskByUuidResult, nil
}

type GetTaskResult struct {
	Data   GetTaskResultData `json:"data"`
	Status string            `json:"status"`
}

type GetTaskResultData struct {
	Task           []model.Task `json:"task"`
	TotalItems     int          `json:"total_items"`
	TotalTaskCount int          `json:"total_task_count"`
}

func (swanClient *SwanClient) SwanGetTasks(limit *int) (*GetTaskResult, error) {
	apiUrl := swanClient.ApiUrl + "/tasks"
	if limit != nil {
		apiUrl = apiUrl + "?limit=" + strconv.Itoa(*limit)
	}
	//logs.GetLogger().Info("Getting My swan tasks info")
	response := HttpGet(apiUrl, swanClient.JwtToken, "")

	if response == "" {
		err := errors.New("failed to get tasks from swan")
		logs.GetLogger().Error(err)
		return nil, err
	}

	//logs.GetLogger().Info(response)

	getTaskResult := &GetTaskResult{}
	err := json.Unmarshal([]byte(response), getTaskResult)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if getTaskResult.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s", getTaskResult.Status)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return getTaskResult, nil
}

func (swanClient *SwanClient) SwanGetAssignedTasksByLimit(limit *int) (*GetTaskResult, error) {
	apiUrl := swanClient.ApiUrl + "/tasks?status=Assigned"
	if limit != nil {
		apiUrl = apiUrl + "&limit=" + strconv.Itoa(*limit)
	}
	//logs.GetLogger().Info("Getting My swan tasks info")
	response := HttpGet(apiUrl, swanClient.JwtToken, "")

	if response == "" {
		err := errors.New("failed to get tasks from swan")
		logs.GetLogger().Error(err)
		return nil, err
	}

	//logs.GetLogger().Info(response)

	getTaskResult := &GetTaskResult{}
	err := json.Unmarshal([]byte(response), getTaskResult)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if getTaskResult.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s", getTaskResult.Status)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return getTaskResult, nil
}

func (swanClient *SwanClient) SwanGetAssignedTasks() ([]model.Task, error) {
	getTaskResult, err := swanClient.SwanGetAssignedTasksByLimit(nil)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if len(getTaskResult.Data.Task) == 0 {
		return nil, nil
	}
	//logs.GetLogger().Info(len(getTaskResult.Data.Task), " ", getTaskResult.Data.TotalTaskCount)

	getTaskResult, err = swanClient.SwanGetAssignedTasksByLimit(&getTaskResult.Data.TotalTaskCount)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if len(getTaskResult.Data.Task) == 0 {
		return nil, nil
	}

	//logs.GetLogger().Info(len(getTaskResult.Data.Task), " ", getTaskResult.Data.TotalTaskCount)

	result := []model.Task{}

	for _, task := range getTaskResult.Data.Task {
		if task.Status == constants.TASK_STATUS_ASSIGNED && task.MinerFid != nil {
			//logs.GetLogger().Info("id: ", task.Id, " task:", task.Status, " miner:", *task.MinerFid)
			result = append(result, task)
		}
	}

	return result, nil
}

type GetOfflineDealsByTaskUuidResult struct {
	Data   GetOfflineDealsByTaskUuidResultData `json:"data"`
	Status string                              `json:"status"`
}
type GetOfflineDealsByTaskUuidResultData struct {
	AverageBid       string              `json:"average_bid"`
	BidCount         int                 `json:"bid_count"`
	DealCompleteRate string              `json:"deal_complete_rate"`
	Deal             []model.OfflineDeal `json:"deal"`
	Miner            model.Miner         `json:"miner"`
	Task             model.Task          `json:"task"`
}

func (swanClient *SwanClient) SwanGetOfflineDealsByTaskUuid(taskUuid string) (*GetOfflineDealsByTaskUuidResult, error) {
	if len(taskUuid) == 0 {
		err := fmt.Errorf("please provide task uuid")
		logs.GetLogger().Error(err)
		return nil, err
	}
	apiUrl := swanClient.ApiUrl + "/tasks/" + taskUuid
	logs.GetLogger().Info("Getting My swan tasks info")
	response := HttpGet(apiUrl, swanClient.JwtToken, "")

	if response == "" {
		err := errors.New("failed to get tasks from swan")
		logs.GetLogger().Error(err)
		return nil, err
	}
	//logs.GetLogger().Info(response)

	getOfflineDealsByTaskUuidResult := &GetOfflineDealsByTaskUuidResult{}
	err := json.Unmarshal([]byte(response), getOfflineDealsByTaskUuidResult)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if getOfflineDealsByTaskUuidResult.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s", getOfflineDealsByTaskUuidResult.Status)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return getOfflineDealsByTaskUuidResult, nil
}

func (swanClient *SwanClient) SwanUpdateTaskByUuid(taskUuid string, minerFid string, csvFilePath string) error {
	apiUrl := swanClient.ApiUrl + "/uuid_tasks/" + taskUuid
	params := map[string]string{}
	params["miner_fid"] = minerFid

	response, err := HttpPutFile(apiUrl, swanClient.JwtToken, params, "file", csvFilePath)
	if err != nil {
		logs.GetLogger().Error(err)
		return err
	}

	if response == "" {
		err := fmt.Errorf("no response from :%s", apiUrl)
		logs.GetLogger().Error(err)
		return err
	}

	status := utils.GetFieldStrFromJson(response, "status")
	if status != constants.SWAN_API_STATUS_SUCCESS {
		message := utils.GetFieldStrFromJson(response, "message")
		err := fmt.Errorf("access:%s failed, status:%s, message:%s", apiUrl, status, message)
		logs.GetLogger().Error(err)
		return err
	}
	data := utils.GetFieldMapFromJson(response, "data")
	filename := data["filename"]

	msg := fmt.Sprintf("access:%s succeeded, file:%s", apiUrl, filename)
	logs.GetLogger().Info(msg)

	return nil
}

func (swanClient *SwanClient) SwanUpdateAssignedTask(taskUuid, status, csvFilePath string) (*SwanCreateTaskResponse, error) {
	apiUrl := swanClient.ApiUrl + "/tasks/" + taskUuid
	logs.GetLogger().Info("Updating Swan task")
	params := map[string]string{}
	params["status"] = status

	response, err := HttpPutFile(apiUrl, swanClient.JwtToken, params, "file", csvFilePath)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	swanCreateTaskResponse := &SwanCreateTaskResponse{}
	err = json.Unmarshal([]byte(response), swanCreateTaskResponse)
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}

	if swanCreateTaskResponse.Status != constants.SWAN_API_STATUS_SUCCESS {
		err := fmt.Errorf("error:%s,%s", swanCreateTaskResponse.Status, swanCreateTaskResponse.Message)
		logs.GetLogger().Error(err)
		return nil, err
	}

	return swanCreateTaskResponse, nil
}
