import React from "react";
import { createRoot } from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import "@/index.css";
import App from "@/App";
import { Toaster } from "@/components/ui/sonner";
import { queryClient } from "@/lib/api";

const container = document.getElementById("root");

const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
      <Toaster richColors closeButton position="bottom-right" />
    </QueryClientProvider>
  </React.StrictMode>
);
