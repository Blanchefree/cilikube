package initialization

import (
	"log"

	"github.com/casbin/casbin/v2"
	"github.com/ciliverse/cilikube/api/v1/handlers"
	"github.com/ciliverse/cilikube/api/v1/routes"
	"github.com/ciliverse/cilikube/configs"
	"github.com/ciliverse/cilikube/internal/service"
	"github.com/ciliverse/cilikube/pkg/k8s"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/runtime"
)

func InitializeServices(k8sManager *k8s.ClusterManager, cfg *configs.Config) *service.AppServices {
	log.Println("正在初始化服务层...")
	resourceFactory := service.NewResourceServiceFactory()
	resourceFactory.InitializeDefaultServices()
	appServices := &service.AppServices{
		ClusterService:     service.NewClusterService(k8sManager),
		InstallerService:   service.NewInstallerService(cfg),
		NodeMetricsService: service.NewNodeMetricsService(),
		PodLogsService:     service.NewPodLogsService(),
		SummaryService:     service.NewSummaryService(),
	}
	// PodExecService 需要 rest.Config
	if activeClient, err := k8sManager.GetActiveClient(); err == nil && activeClient != nil {
		appServices.PodExecService = service.NewPodExecService(activeClient.Config)
	} else {
		appServices.PodExecService = nil // 或可 panic/log
	}
	initializeResourceService(resourceFactory, "nodes", &appServices.NodeService)
	initializeResourceService(resourceFactory, "pods", &appServices.PodService)
	initializeResourceService(resourceFactory, "deployments", &appServices.DeploymentService)
	initializeResourceService(resourceFactory, "services", &appServices.ServiceService)
	initializeResourceService(resourceFactory, "daemonsets", &appServices.DaemonSetService)
	initializeResourceService(resourceFactory, "ingresses", &appServices.IngressService)
	initializeResourceService(resourceFactory, "configmaps", &appServices.ConfigMapService)
	initializeResourceService(resourceFactory, "secrets", &appServices.SecretService)
	initializeResourceService(resourceFactory, "persistentvolumeclaims", &appServices.PVCService)
	initializeResourceService(resourceFactory, "persistentvolumes", &appServices.PVService)
	initializeResourceService(resourceFactory, "statefulsets", &appServices.StatefulSetService)
	initializeResourceService(resourceFactory, "namespaces", &appServices.NamespaceService)
	return appServices
}

func initializeResourceService[T runtime.Object](factory *service.ResourceServiceFactory, resourceName string, serviceField *service.ResourceService[T]) {
	if svc, ok := factory.GetService(resourceName).(service.ResourceService[T]); ok {
		*serviceField = svc
	} else {
		log.Fatalf("初始化 %s 服务失败: 类型断言失败或服务未找到", resourceName)
	}
}

// InitializeHandlers 函数
func InitializeHandlers(router *gin.RouterGroup, services *service.AppServices, k8sManager *k8s.ClusterManager) {
	// --- 1. 注册非资源类的特殊路由 ---
	routes.RegisterAuthRoutes(router.Group("/auth"))
	routes.RegisterClusterRoutes(router, handlers.NewClusterHandler(services.ClusterService))
	routes.RegisterInstallerRoutes(router, handlers.NewInstallerHandler(services.InstallerService))
	routes.KubernetesProxyRoutes(router, handlers.NewProxyHandler(k8sManager))
	
	// --- 注册汇总路由 ---
	routes.RegisterSummaryRoutes(router, handlers.NewSummaryHandler(services.SummaryService, k8sManager))

	// --- 2. 创建所有资源的 Handler 实例 ---
	nodesHandler := handlers.NewResourceHandler(services.NodeService, k8sManager, "nodes")
	pvHandler := handlers.NewResourceHandler(services.PVService, k8sManager, "persistentvolumes")
	namespacesHandler := handlers.NewResourceHandler(services.NamespaceService, k8sManager, "namespaces")
	podsHandler := handlers.NewResourceHandler(services.PodService, k8sManager, "pods")
	deploymentsHandler := handlers.NewResourceHandler(services.DeploymentService, k8sManager, "deployments")
	servicesHandler := handlers.NewResourceHandler(services.ServiceService, k8sManager, "services")
	daemonsetsHandler := handlers.NewResourceHandler(services.DaemonSetService, k8sManager, "daemonsets")
	ingressesHandler := handlers.NewResourceHandler(services.IngressService, k8sManager, "ingresses")
	configmapsHandler := handlers.NewResourceHandler(services.ConfigMapService, k8sManager, "configmaps")
	secretsHandler := handlers.NewResourceHandler(services.SecretService, k8sManager, "secrets")
	pvcHandler := handlers.NewResourceHandler(services.PVCService, k8sManager, "persistentvolumeclaims")
	statefulsetsHandler := handlers.NewResourceHandler(services.StatefulSetService, k8sManager, "statefulsets")
	nodeMetricsHandler := handlers.NewNodeMetricsHandler(services.NodeMetricsService, k8sManager)

	// Pod 日志与终端 Handler
	podLogsHandler := handlers.NewPodLogsHandler(services.PodLogsService, k8sManager)
	podExecHandler := handlers.NewPodExecHandler(services.PodExecService, k8sManager)

	// a. 集群作用域的资源
	nodesRoutes := router.Group("/nodes")
	{
		nodesRoutes.GET("", nodesHandler.List)
		nodesRoutes.POST("", nodesHandler.Create)
		// 针对单个节点的操作
		nodeMemberRoutes := nodesRoutes.Group("/:name")
		{
			nodeMemberRoutes.GET("", nodesHandler.Get)
			nodeMemberRoutes.PUT("", nodesHandler.Update)
			nodeMemberRoutes.DELETE("", nodesHandler.Delete)
			nodeMemberRoutes.GET("/watch", nodesHandler.Watch)
			// 注册 metrics 子路由
			nodeMemberRoutes.GET("/metrics", nodeMetricsHandler.GetNodeMetrics)
		}
	}

	pvRoutes := router.Group("/persistentvolumes")
	{
		pvRoutes.GET("", pvHandler.List)
		pvRoutes.POST("", pvHandler.Create)
		pvRoutes.GET("/:name", pvHandler.Get)
		pvRoutes.PUT("/:name", pvHandler.Update)
		pvRoutes.DELETE("/:name", pvHandler.Delete)
		pvRoutes.GET("/:name/watch", pvHandler.Watch)
	}

	podsTopLevelRoutes := router.Group("/pods")
	{
		podsTopLevelRoutes.GET("", podsHandler.List)
	}

	// b. Namespace 资源本身，以及所有嵌套在其下的资源
	namespacesRoutes := router.Group("/namespaces")
	{
		namespacesRoutes.GET("", namespacesHandler.List)
		namespacesRoutes.POST("", namespacesHandler.Create)

		// 针对单个 Namespace 的操作
		nsMemberRoutes := namespacesRoutes.Group(":namespace")
		{
			nsMemberRoutes.GET("", namespacesHandler.Get)
			nsMemberRoutes.PUT("", namespacesHandler.Update)
			nsMemberRoutes.DELETE("", namespacesHandler.Delete)

			// 嵌套资源
			registerResourceInNamespace(nsMemberRoutes, "pods", podsHandler)
			registerResourceInNamespace(nsMemberRoutes, "deployments", deploymentsHandler)
			registerResourceInNamespace(nsMemberRoutes, "services", servicesHandler)
			registerResourceInNamespace(nsMemberRoutes, "daemonsets", daemonsetsHandler)
			registerResourceInNamespace(nsMemberRoutes, "ingresses", ingressesHandler)
			registerResourceInNamespace(nsMemberRoutes, "configmaps", configmapsHandler)
			registerResourceInNamespace(nsMemberRoutes, "secrets", secretsHandler)
			registerResourceInNamespace(nsMemberRoutes, "persistentvolumeclaims", pvcHandler)
			registerResourceInNamespace(nsMemberRoutes, "statefulsets", statefulsetsHandler)

			// 新增：Pod 日志与终端路由
			podsMemberRoutes := nsMemberRoutes.Group("/pods/:name")
			{
				podsMemberRoutes.GET("/logs", podLogsHandler.GetPodLogs)
				podsMemberRoutes.GET("/exec", podExecHandler.ExecPod)
			}
		}
	}
}
func registerResourceInNamespace[T runtime.Object](nsRouter *gin.RouterGroup, resourceName string, handler *handlers.ResourceHandler[T]) {
	if handler == nil {
		return
	}

	resourceRoutes := nsRouter.Group("/" + resourceName)
	{
		resourceRoutes.GET("", handler.List)
		resourceRoutes.POST("", handler.Create)

		memberRoutes := resourceRoutes.Group("/:name")
		{
			memberRoutes.GET("", handler.Get)
			memberRoutes.PUT("", handler.Update)
			memberRoutes.DELETE("", handler.Delete)
			memberRoutes.GET("/watch", handler.Watch)
		}
	}
}

// SetupRouter 设置并返回 Gin 引擎
func SetupRouter(cfg *configs.Config, services *service.AppServices, k8sManager *k8s.ClusterManager, e *casbin.Enforcer) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery(), gin.Logger())

	// 配置自定义 CORS 中间件，允许所有需要的头部
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	apiV1 := router.Group("/api/v1")
	{
		InitializeHandlers(apiV1, services, k8sManager)
	}

	return router
}
