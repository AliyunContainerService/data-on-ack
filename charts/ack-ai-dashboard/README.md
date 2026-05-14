# ack-ai-dashboard

`ack-ai-dashboard` is an operations management console for cluster administrators. It provides cluster monitoring dashboards, dataset management and acceleration, user-level resource quota allocation, job listings, and cost estimation to help users quickly set up and manage a machine learning environment on Kubernetes clusters.

## Prerequisites

Before installing `ack-ai-dashboard`, ensure the following:

- This component cannot work independently. It depends on **ack-arena** and **ack-arena-exporter**.
- The **Job List** monitoring page requires separate permission configuration.
- Cluster type: **ACK Pro edition cluster**.

## Installation

To install `ack-ai-dashboard`:

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. Select an ACK Pro cluster and navigate to **Applications** → **AI & Big Data** in the left sidebar.
3. Click the **One-click Deploy** button.
4. Check **AI-Dashboard** console. In the pop-up dialog, follow the instructions to configure permissions and bind the administrator Alibaba Cloud account UID. See the **Installing the AI-Dashboard Console** section below for detailed steps.
5. Check **O&M Components** and click **Deploy KubeAI** at the bottom of the page.

## Installing the AI-Dashboard Console

To secure access to `ack-ai-dashboard`, the component integrates with the Alibaba Cloud RAM authentication system. `ack-ai-dashboard` acts as a RAM WebApp and obtains an OAuth access token using a RAM-issued client identity.

Before installation, you need to provide:

1. The Alibaba Cloud UID of an administrator account with access permissions.
2. The `clientID` and `clientSecret` issued by RAM for the WebApp.

To obtain these:

1. Go to the [RAM User Management](https://ram.console.aliyun.com/users) page and create an administrator account.
2. Go to the [RAM Application Management](https://ram.console.aliyun.com/applications) page, create an application, and refer to the [RAM official documentation](https://help.aliyun.com/document_detail/93693.html) for guidance.
3. On the application details page, add the Alibaba Cloud UID to the authorized scope.
4. Create a secret for `ack-ai-dashboard` to access RAM. At this point, `ack-ai-dashboard` is ready to be installed.
5. After installation and configuring the access method for `ack-ai-dashboard`, configure the WebApp callback URL in the RAM console.

## Access Methods

Four reference access methods are currently available. Refer to the `ack-ai-dashboard` access configuration documentation for details.
