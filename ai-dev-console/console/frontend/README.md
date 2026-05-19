# AI Dev Console Frontend

React-based frontend application for the AI Dev Console, built with Ant Design Pro and UmiJS.

## Prerequisites

- **Node.js**: 16.x or later
- **npm**: 7.x or later (or yarn)
- **Operating System**: macOS, Linux, or Windows

## Quick Start

### Installation

```bash
# Install dependencies
npm install --legacy-peer-deps

# Or using yarn
yarn install
```

> **Note**: We use `--legacy-peer-deps` flag to handle dependency conflicts. This is configured in `.npmrc` file.

### Development

```bash
# Start development server
npm start

# Start without mock data
npm run start:no-mock

# Start without UI
npm run start:no-ui
```

The development server will start at `http://localhost:8000` (default port).

### Build

```bash
# Production build
npm run build

# Build with bundle analysis
npm run build:analyze

# Stage build
npm run build:stage
```

Build output will be in the `dist/` directory.

### Verify Build

```bash
# Verify build artifacts
npm run verify
```

## Project Structure

```
frontend/
├── config/              # UmiJS configuration files
│   ├── config.js        # Main configuration
│   ├── config.default.js
│   └── ...
├── src/
│   ├── components/      # Reusable components
│   ├── layouts/         # Layout components
│   ├── pages/           # Page components
│   ├── services/        # API services
│   ├── utils/           # Utility functions
│   └── locales/         # Internationalization
├── public/              # Static assets
├── mock/                # Mock data for development
└── dist/                # Build output (generated)
```

## Available Scripts

### Development
- `npm start` - Start development server
- `npm run start:no-mock` - Start without mock data
- `npm run start:no-ui` - Start without Umi UI

### Build
- `npm run build` - Production build
- `npm run build:analyze` - Build with bundle analysis
- `npm run build:stage` - Stage environment build
- `npm run eflops-build` - Build for eflops environment

### Code Quality
- `npm run lint` - Run all linters
- `npm run lint:js` - Lint JavaScript/TypeScript files
- `npm run lint:style` - Lint CSS/Less files
- `npm run lint:fix` - Auto-fix linting errors
- `npm run prettier` - Format code with Prettier

### Testing
- `npm test` - Run tests
- `npm run test:all` - Run all tests
- `npm run test:component` - Test components

### Utilities
- `npm run clean` - Clean build artifacts and cache
- `npm run clean:all` - Clean everything including node_modules
- `npm run verify` - Verify build artifacts
- `npm run analyze` - Analyze bundle size

## Configuration

### Environment Variables

- `UMI_ENV` - Environment (default, eflops)
- `NODE_ENV` - Node environment (development, production)
- `MOCK` - Enable/disable mock data (none to disable)
- `UMI_UI` - Enable/disable Umi UI (none to disable)

### Proxy Configuration

The development server proxies API requests to the backend:

- `/api/v1` → `http://127.0.0.1:9090/`
- `/notebook/` → `http://127.0.0.1:9090/`
- `/mlflow` → `http://127.0.0.1:9090/`

See `config/config.js` for detailed proxy configuration.

## Development Guidelines

### Code Style

1. **Linting**: Run `npm run lint` before committing
2. **Formatting**: Use Prettier for code formatting
3. **TypeScript**: Use TypeScript for new components when possible
4. **Components**: Follow Ant Design Pro component patterns

### Best Practices

1. **Component Structure**: Keep components small and focused
2. **State Management**: Use DVA for complex state management
3. **API Calls**: Use services layer for API calls
4. **Internationalization**: Use locale files for all user-facing text
5. **Error Handling**: Implement proper error boundaries

### Performance Optimization

1. **Code Splitting**: Use dynamic imports for large components
2. **Lazy Loading**: Implement route-based code splitting
3. **Bundle Analysis**: Regularly run `npm run build:analyze`
4. **Image Optimization**: Optimize images before adding to assets

## Troubleshooting

### Build Issues

**Problem**: Build fails with dependency errors
```bash
# Solution: Clean and reinstall
npm run clean:all
npm install --legacy-peer-deps
```

**Problem**: Build is slow
```bash
# Solution: Clear cache
npm run clean
npm start
```

**Problem**: Port already in use
```bash
# Solution: Kill process on port 8000 or use different port
PORT=8001 npm start
```

### Development Issues

**Problem**: Hot reload not working
```bash
# Solution: Clear cache and restart
rm -rf .umi .umi-production
npm start
```

**Problem**: Styles not updating
```bash
# Solution: Clear style cache
rm -rf node_modules/.cache
npm start
```

## CI/CD Integration

### Build in CI

```bash
# Install dependencies
npm ci --legacy-peer-deps

# Run linting
npm run lint

# Build
npm run build

# Verify build
npm run verify
```

### Docker Build

The frontend is built as part of the Docker image. See the main `Makefile` for build commands.

## Contributing

1. Follow the code style guidelines
2. Run linting before committing
3. Write tests for new features
4. Update documentation as needed

## License

Apache License 2.0
