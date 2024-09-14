export default {
  title: "Cloud Native AI",
  define: {
    environment: "dlc"
  },
  routes: [
    {
      path: "/login",
      component: "@/layouts/LoginLayout",
    },
    {
      path: "/",
      component: "@/layouts/SecurityLayout",
      routes: [
        {
          path: "/",
          component: "@/layouts/BasicLayout",
          authority: ["admin", "user"],
          routes: [
            {
              path: "/",
              redirect: "/cluster"
            },
            {
              path: "/cluster",
              name: "cluster",
              icon: "home",
              component: "@/pages/ClusterInfo"
            },
            {
              path: "/datasheets",
              name: "datasheets",
              icon: "database",
              component: "@/pages/DataSheets"
            },
            {
              path: "/notebooks",
              name: "notebooks",
              icon: "BookOutlined",
              component: "@/pages/Notebooks"
            },
            {
              path: "/job-submit",
              name: "job-submit",
              icon: "edit",
              component: "@/pages/JobCreate"
            },
            {
              path: "/jobs",
              name: "jobs",
              icon: "unordered-list",
              component: "@/pages/Jobs"
            },
            {
              path: "/crons",
              name: "crons",
              icon: "ClockCircleOutlined",
              component: "@/pages/Crons"
            },
            {
              path: "/notebooks/notebook-create",
              component: "@/pages/NotebookCreate"
            },
            {
              name: "modelManage",
              path: "/modelManage",
              icon: "database",
              routes: [
                {
                  path: "/modelManage",
                  redirect: "models"
                },
                {
                  path: "models",
                  component: "@/pages/ModelManage/RegisteredModelList",
                },
                {
                  path: "models/:name",
                  component: "@/pages/ModelManage/RegisteredModelPage"
                },
                {
                  path: "models/:name/versions/:version",
                  component: "@/pages/ModelManage/ModelVersionPage"
                }
              ]
            },
            // {
            //   path: "/evaluateJobs",
            //   name: "evaluateJobs",
            //   icon: "BookOutlined",
            //   component: "@/pages/EvaluateJob"
            // },
            {
              // use mock url force antd to produce link object
              path: "http://www.kubeflow.org",
              target: '_blank',
              name: "Kubeflow Pipelines",
              icon: "RiseOutlined"
            },
            // {
            //   path: "/evaluateJobs/create",
            //   component: "@/pages/EvaluateJobCreate"
            // },
            // {
            //   path: "/evaluateJobs/metrics",
            //   component: "@/pages/EvaluateJobMetrics"
            // },
            // {
            //   path: "/evaluateJobs/compare",
            //   component: "@/pages/EvaluateJobCompare"
            // },
            {
              path: "/jobs/detail",
              component: "@/pages/JobDetail"
            },
            {
              path: "/jobs/job-create",
              component: "@/pages/JobCreate"
            },
            {
              path: "/datasheets/data-config",
              component: "@/pages/DataConfig"
            },
            {
              path: "/datasheets/git-config",
              component: "@/pages/GitConfig"
            },
            {
              path: "/crons/history",
              component: "@/pages/CronHistory"
            },
            {
              component: "@/pages/404"
            },
            
          ]
        }
      ]
    }
  ]
};
