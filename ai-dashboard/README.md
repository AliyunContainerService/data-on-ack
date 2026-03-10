# ai-dashboard

AI Dashboard is an operations console for cluster administrators. It provides cluster monitoring, dataset management, user-level resource quota allocation, job listings, and cost estimation to help users quickly set up and manage a machine learning environment on Kubernetes.

## Components

- **Backend** – Spring Boot application (JDK 11 + Maven 3.6+)
- **Frontend** – Vue.js application with Element UI

---

## Backend

### Requirements

- JDK 11
- Maven 3.6+

### Build & Run

```bash
# Clone the project
git clone https://github.com/AliyunContainerService/data-on-ack.git

# Enter the backend directory
cd data-on-ack/ai-dashboard/backend

# Run with Maven
mvn spring-boot:run
```

---

## Frontend

### Requirements

- Node.js 12+
- npm 6+

### Development

```bash
# Enter the frontend directory
cd data-on-ack/ai-dashboard/frontend

# Install dependencies
npm install

# Start development server (opens http://localhost:9528 automatically)
npm run dev
```

### Build

```bash
# Build for staging environment
npm run build:stage

# Build for production environment
npm run build:prod
```

### Advanced

```bash
# Preview the production build
npm run preview

# Preview with static resource analysis report
npm run preview -- --report

# Check code formatting
npm run lint

# Check and auto-fix code formatting
npm run lint -- --fix
```

