# shiet

shiet is what I use to summarize my time spent on various activities each pay period. I use it to import my calendar, categorize events, fill in any gaps, and then export a report that I can use for my timesheet.

## 🛠️ Development

Run the app in development mode with hot reload:

```bash
wails dev
```

The frontend dev server runs on http://localhost:5173 with Vite's fast HMR.

## 🏗️ Building

### Current Platform
```bash
wails build
# or
./scripts/build.sh
```

### Cross-Platform Builds
```bash
# Build for all platforms
./scripts/build-all.sh

# Individual platforms
./scripts/build-windows.sh      # Windows AMD64
./scripts/build-linux.sh         # Linux AMD64
./scripts/build-macos-arm.sh     # macOS Apple Silicon
./scripts/build-macos-intel.sh   # macOS Intel
./scripts/build-macos-universal.sh  # macOS Universal Binary
```

Built applications will be in `build/bin/`

## 🎨 shadcn/ui Components

This template includes pre-configured shadcn/ui components:
- Button
- Input
- Label
- Card

Add more components:
```bash
npx shadcn@latest add [component-name]
```

Browse components at [ui.shadcn.com](https://ui.shadcn.com/)

## 📁 Project Structure

```
.
├── app.tmpl.go              # Main application logic
├── main.tmpl.go             # Entry point
├── frontend/
│   ├── src/
│   │   ├── App.tsx          # Main React component
│   │   ├── components/ui/   # shadcn/ui components
│   │   └── lib/utils.ts     # Utility functions
│   ├── vite.config.ts       # Vite configuration
│   └── package.json         # Frontend dependencies
└── scripts/                 # Build scripts
```

## 🔧 Configuration

Project configuration is in `wails.json` (auto-generated on `wails init`). 

See [Wails documentation](https://wails.io/docs/reference/project-config) for all options.

## 📚 Learn More

- [Wails Documentation](https://wails.io/docs/introduction)
- [React Documentation](https://react.dev/)
- [Vite Documentation](https://vitejs.dev/)
- [Tailwind CSS Documentation](https://tailwindcss.com/)
- [shadcn/ui Documentation](https://ui.shadcn.com/)

## 📝 License

This template is available as open source under the terms of the MIT License.
