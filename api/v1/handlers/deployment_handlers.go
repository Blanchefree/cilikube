package handlers

import (
	"io"
	appsv1 "k8s.io/api/apps/v1"
	"net/http"
	"strconv"
	"strings"

	"github.com/ciliverse/cilikube/api/v1/models"
	"github.com/ciliverse/cilikube/internal/service"
	"github.com/ciliverse/cilikube/pkg/utils"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/api/errors"
)

// DeploymentHandler ...
type DeploymentHandler struct {
	service *service.DeploymentService
}

// NewDeploymentHandler ...
func NewDeploymentHandler(svc *service.DeploymentService) *DeploymentHandler {
	return &DeploymentHandler{service: svc}
}

// ListDeployments ...
func (h *DeploymentHandler) ListDeployments(c *gin.Context) {
	namespace := c.Param("namespace")
	// 1. 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	// 2. 调用服务层获取Deployment列表
	deployments, err := h.service.List(namespace)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取Deployment列表失败: "+err.Error())
		return
	}

	// 无数据的话slice未初始化，返回前端会是null，导致前端报错，特处理。如果前端可以处理，这个判断可删除
	if len(deployments.Items) == 0 {
		deployments.Items = make([]appsv1.Deployment, 0)
	}

	// 3. 返回结果
	respondSuccess(c, http.StatusOK, deployments)

}

// CreateDeployment ...
func (h *DeploymentHandler) CreateDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	// 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	contentType := c.ContentType()
	var data []byte
	var err error

	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "x-yaml") || strings.Contains(contentType, "json") {
		data, err = io.ReadAll(c.Request.Body)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "获取请求参数失败: "+err.Error())
			return
		}
	} else {
		respondError(c, http.StatusUnsupportedMediaType, "不支持的 Content-Type，请使用 application/json 或 application/yaml")
		return
	}

	// 解析为 Deployment 对象
	deployment, err := utils.ParseDeploymentFromFile(data)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "解析Deployment对象失败: "+err.Error())
		return
	}

	if deployment.Namespace == "" {
		deployment.Namespace = namespace
	}

	// 调用服务层创建Deployment
	createdDeployment, err := h.service.Create(namespace, deployment)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			respondError(c, http.StatusConflict, "Deployment已存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "创建Deployment失败: "+err.Error())
		return
	}

	respondSuccess(c, http.StatusOK, models.ToDeploymentResponse(createdDeployment))
}

// GetDeployment ...
func (h *DeploymentHandler) GetDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	// 1. 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	if !utils.ValidateResourceName(name) {
		respondError(c, http.StatusBadRequest, "无效的Deployment名称格式")
		return
	}

	// 2. 调用服务层获取Deployment详情
	deployment, err := h.service.Get(namespace, name)
	if err != nil {
		if errors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, "Deployment不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "获取Deployment失败: "+err.Error())
		return
	}

	// 3. 返回结果
	respondSuccess(c, http.StatusOK, models.ToDeploymentResponse(deployment))

}

// UpdateDeployment ...
func (h *DeploymentHandler) UpdateDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	// 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	if !utils.ValidateResourceName(name) {
		respondError(c, http.StatusBadRequest, "无效的Deployment名称格式")
		return
	}

	contentType := c.ContentType()
	var data []byte
	var err error

	if strings.Contains(contentType, "yaml") || strings.Contains(contentType, "x-yaml") || strings.Contains(contentType, "json") {
		data, err = io.ReadAll(c.Request.Body)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "获取请求参数失败: "+err.Error())
			return
		}
	} else {
		respondError(c, http.StatusUnsupportedMediaType, "不支持的 Content-Type，请使用 application/json 或 application/yaml")
		return
	}

	// 解析为 Deployment 对象
	updateDeployment, err := utils.ParseDeploymentFromFile(data)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "解析Deployment对象失败: "+err.Error())
		return
	}

	if updateDeployment.Namespace == "" {
		updateDeployment.Namespace = namespace
	}

	// 调用服务层更新Deployment
	resultDeployment, err := h.service.Update(namespace, name, updateDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, "Deployment不存在 (可能在更新期间被删除)")
			return
		}
		if errors.IsConflict(err) {
			respondError(c, http.StatusConflict, "Deployment已被修改，请重试 (ResourceVersion conflict)")
			return
		}
		respondError(c, http.StatusInternalServerError, "更新Deployment失败: "+err.Error())
		return
	}

	respondSuccess(c, http.StatusOK, models.ToDeploymentResponse(resultDeployment))
}

// DeleteDeployment ...
func (h *DeploymentHandler) DeleteDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	// 1. 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	if !utils.ValidateResourceName(name) {
		respondError(c, http.StatusBadRequest, "无效的Deployment名称格式")
		return
	}

	if err := h.service.Delete(namespace, name); err != nil {
		if errors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, "Deployment不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "删除Deployment失败: "+err.Error())
		return
	}

	// 3. 返回结果
	respondSuccess(c, http.StatusOK, gin.H{"message": "删除成功"})
}

// WatchDeployments ...
func (h *DeploymentHandler) WatchDeployments(c *gin.Context) {}

// ScaleDeployment ...
func (h *DeploymentHandler) ScaleDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req models.ScaleDeploymentRequest

	// 1. 参数校验
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "无效的Replicas格式: "+err.Error())
		return
	}

	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	if !utils.ValidateResourceName(name) {
		respondError(c, http.StatusBadRequest, "无效的Deployment名称格式")
		return
	}

	// 2. 调用服务层修改Deployment的副本数
	deployment, err := h.service.Scale(namespace, name, req.Replicas)
	if err != nil {
		if errors.IsNotFound(err) {
			respondError(c, http.StatusNotFound, "Deployment不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "修改Deployment的副本数失败: "+err.Error())
		return
	}

	// 3. 返回结果
	respondSuccess(c, http.StatusOK, models.ToDeploymentResponse(deployment))
}

// GetDeploymentPods ...
func (h *DeploymentHandler) GetDeploymentPods(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	// 1. 参数校验
	if !utils.ValidateNamespace(namespace) {
		respondError(c, http.StatusBadRequest, "无效的命名空间格式")
		return
	}

	if !utils.ValidateResourceName(name) {
		respondError(c, http.StatusBadRequest, "无效的Deployment名称格式")
		return
	}

	limitStr := c.DefaultQuery("limit", "500") // Sensible default limit
	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit <= 0 {
		limit = 500 // Fallback
	}

	pods, err := h.service.PodList(namespace, name, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "获取Pod列表失败: "+err.Error())
		return
	}

	response := models.PodListResponse{
		Items: make([]models.PodResponse, 0, len(pods.Items)),
		// Total reflects items *in this batch*. K8s list doesn't give total count easily.
		Total: len(pods.Items),
	}

	for _, pod := range pods.Items {
		response.Items = append(response.Items, models.ToPodResponse(&pod))
	}

	respondSuccess(c, http.StatusOK, response)
}
