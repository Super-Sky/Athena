// main.tsx boots the standalone control-plane web application and mounts the root React tree.
// main.tsx 负责启动独立控制面 Web 应用并挂载根 React 树。
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

// swagger-ui-react still emits legacy lifecycle warnings under StrictMode in dev.
// The control-plane console disables StrictMode at the root to keep embedded Swagger diagnostics clean.
ReactDOM.createRoot(document.getElementById("root")!).render(<App />);
