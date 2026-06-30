import {
  Check,
  LoaderCircle,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useAIModels,
  useClassifyAIEndpoint,
  useClearAIModel,
  useDiscoverLocalAIEndpoints,
  useSaveAIConfig,
  useSaveAIEndpoint,
  useSaveAIModel,
  useSetting,
  useValidateAIConfig,
} from "@/lib/api";
import type { AIEndpoint } from "@/lib/api/types";
import { aiEndpointsMatch } from "@/lib/ai/endpoints";
import { SettingBlock } from "./SettingBlock";

function parseJsonSetting<T>(raw: string | null | undefined, fallback: T): T {
  if (!raw) {
    return fallback;
  }

  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export function AIModelSettings() {
  const baseURLSetting = useSetting("ai.base_url");
  const modelSetting = useSetting("ai.model");
  const savedBaseURL = useMemo(
    () => parseJsonSetting(baseURLSetting.data, ""),
    [baseURLSetting.data],
  );
  const savedModel = useMemo(
    () => parseJsonSetting(modelSetting.data, ""),
    [modelSetting.data],
  );

  const [baseURL, setBaseURL] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState("");
  const [validationMessage, setValidationMessage] = useState<string | null>(
    null,
  );
  const pendingModelPickRef = useRef<{
    discoveredModel?: string;
  } | null>(null);

  const discovery = useDiscoverLocalAIEndpoints();
  const activeBaseURL = baseURL || savedBaseURL;
  const classify = useClassifyAIEndpoint(activeBaseURL);
  const modelsQuery = useAIModels(activeBaseURL, apiKey);
  const validate = useValidateAIConfig(activeBaseURL, apiKey, model);
  const saveEndpoint = useSaveAIEndpoint();
  const saveModel = useSaveAIModel();
  const clearModel = useClearAIModel();
  const saveConfig = useSaveAIConfig();

  const modelSavedForActiveEndpoint = Boolean(
    savedModel &&
      savedBaseURL &&
      aiEndpointsMatch(activeBaseURL, savedBaseURL),
  );

  const models =
    modelsQuery.data ??
    (modelSavedForActiveEndpoint && savedModel ? [savedModel] : []);

  // Restore persisted settings when the panel opens.
  useEffect(() => {
    if (savedBaseURL) {
      setBaseURL(savedBaseURL);
    }
  }, [savedBaseURL]);

  // Restore a saved model only when it belongs to the active endpoint and
  // appears in that endpoint's loaded model list.
  useEffect(() => {
    if (
      !savedModel ||
      !modelSavedForActiveEndpoint ||
      !modelsQuery.data?.includes(savedModel) ||
      model
    ) {
      return;
    }
    setModel(savedModel);
  }, [model, modelSavedForActiveEndpoint, modelsQuery.data, savedModel]);

  const persistEndpoint = useCallback(
    async (nextBaseURL: string) => {
      const trimmedBaseURL = nextBaseURL.trim();
      if (!trimmedBaseURL) {
        return;
      }
      await saveEndpoint.mutateAsync(trimmedBaseURL);
    },
    [saveEndpoint],
  );

  const persistModel = useCallback(
    async (nextModel: string) => {
      const trimmedModel = nextModel.trim();
      if (!trimmedModel) {
        return;
      }
      await saveModel.mutateAsync(trimmedModel);
    },
    [saveModel],
  );

  const resetModelSelection = useCallback(async () => {
    setModel("");
    await clearModel.mutateAsync();
  }, [clearModel]);

  // After selecting a reachable endpoint, pick a model once its list loads.
  useEffect(() => {
    if (!pendingModelPickRef.current || modelsQuery.isFetching) {
      return;
    }

    const { discoveredModel } = pendingModelPickRef.current;
    pendingModelPickRef.current = null;

    if (modelsQuery.isError || !modelsQuery.data?.length) {
      void resetModelSelection();
      return;
    }

    const nextModels = modelsQuery.data;
    const preferredModel =
      (discoveredModel &&
        nextModels.includes(discoveredModel) &&
        discoveredModel) ||
      nextModels[0] ||
      "";

    if (preferredModel) {
      setModel(preferredModel);
      void persistModel(preferredModel);
    } else {
      void resetModelSelection();
    }
  }, [
    modelsQuery.data,
    modelsQuery.isError,
    modelsQuery.isFetching,
    persistModel,
    resetModelSelection,
  ]);

  const classification = useMemo(() => {
    if (!activeBaseURL.trim()) {
      return null;
    }
    return classify.data ?? null;
  }, [activeBaseURL, classify.data]);

  const isSavedEndpoint = useMemo(
    () => Boolean(savedBaseURL && aiEndpointsMatch(activeBaseURL, savedBaseURL)),
    [activeBaseURL, savedBaseURL],
  );

  const isSavedModel = useMemo(
    () => Boolean(savedModel && model === savedModel && modelSavedForActiveEndpoint),
    [model, modelSavedForActiveEndpoint, savedModel],
  );

  const refreshModels = async () => {
    const result = await modelsQuery.refetch();
    const nextModels = result.data ?? [];
    if (nextModels.length === 0) {
      await resetModelSelection();
      return;
    }
    if (model && !nextModels.includes(model)) {
      const fallbackModel = nextModels[0];
      setModel(fallbackModel);
      await persistModel(fallbackModel);
    }
  };

  const handleSelectEndpoint = async (endpoint: AIEndpoint) => {
    setValidationMessage(null);
    setBaseURL(endpoint.baseUrl);
    setModel("");
    await persistEndpoint(endpoint.baseUrl);

    if (!endpoint.running) {
      pendingModelPickRef.current = null;
      await resetModelSelection();
      setValidationMessage(
        `${endpoint.name} is not running. Start it and scan again to load models.`,
      );
      return;
    }

    pendingModelPickRef.current = {
      discoveredModel: endpoint.models?.[0],
    };
  };

  const handleModelChange = (nextModel: string) => {
    setModel(nextModel);
    void persistModel(nextModel);
  };

  const handleValidate = async () => {
    const result = await validate.refetch();
    if (result.error) {
      setValidationMessage(
        result.error instanceof Error
          ? result.error.message
          : "Unable to validate configuration",
      );
      return;
    }

    const validation = result.data;
    if (!validation) {
      return;
    }

    setValidationMessage(validation.message);
    if (validation.ok) {
      await saveConfig.mutateAsync({ baseURL: activeBaseURL, model });
    }
  };

  const isBusy =
    discovery.isLoading ||
    modelsQuery.isFetching ||
    validate.isFetching ||
    saveEndpoint.isPending ||
    saveModel.isPending ||
    clearModel.isPending ||
    saveConfig.isPending;

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <SettingBlock
        title="AI Model"
        description="Bring your own model. Point Clockr at a local runtime or a custom OpenAI-compatible endpoint for categorization suggestions."
      >
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-3">
            <Label className="text-xs">Detected endpoints</Label>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={discovery.isFetching}
              onClick={() => void discovery.refetch()}
            >
              <RefreshCw
                className={`size-4 ${discovery.isFetching ? "animate-spin" : ""}`}
              />
              Scan
            </Button>
          </div>

          <div className="space-y-2">
            {(discovery.data ?? []).map((endpoint) => (
              <button
                key={endpoint.baseUrl}
                type="button"
                className={`flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left transition-colors ${
                  aiEndpointsMatch(activeBaseURL, endpoint.baseUrl)
                    ? "border-primary bg-primary/5"
                    : "border-border hover:bg-muted/50"
                }`}
                onClick={() => void handleSelectEndpoint(endpoint)}
              >
                <div>
                  <div className="flex items-center gap-2 text-sm font-medium">
                    <span>{endpoint.name}</span>
                    <span
                      className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${
                        endpoint.running
                          ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                          : "bg-muted text-muted-foreground"
                      }`}
                    >
                      {endpoint.running ? "Running" : "Not running"}
                    </span>
                    {isSavedEndpoint &&
                    aiEndpointsMatch(savedBaseURL, endpoint.baseUrl) ? (
                      <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                        Saved
                      </span>
                    ) : null}
                  </div>
                  <p className="mt-0.5 font-mono text-xs text-muted-foreground">
                    {endpoint.baseUrl}
                  </p>
                </div>
              </button>
            ))}
          </div>
        </div>
      </SettingBlock>

      <SettingBlock
        title="Connection"
        description="Configure the OpenAI-compatible base URL and model Clockr should use."
      >
        <div className="grid gap-3">
          <div className="grid gap-1.5">
            <Label htmlFor="ai-base-url" className="text-xs">
              Base URL
            </Label>
            <Input
              id="ai-base-url"
              className="font-mono"
              value={baseURL}
              onChange={(event) => setBaseURL(event.target.value)}
              onBlur={() => {
                void persistEndpoint(baseURL);
              }}
              placeholder="http://127.0.0.1:11434/v1"
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="ai-api-key" className="text-xs">
              API key <span className="text-muted-foreground">(optional)</span>
            </Label>
            <Input
              id="ai-api-key"
              type="password"
              className="font-mono"
              value={apiKey}
              onChange={(event) => setApiKey(event.target.value)}
              placeholder="Not required for local models"
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <div className="grid gap-1.5">
              <Label htmlFor="ai-model" className="text-xs">
                Model
              </Label>
              <Select
                value={model || undefined}
                onValueChange={handleModelChange}
              >
                <SelectTrigger id="ai-model" className="w-full bg-background">
                  <SelectValue placeholder="Select a model" />
                </SelectTrigger>
                <SelectContent>
                  {models.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={!activeBaseURL.trim() || isBusy}
                onClick={() => void refreshModels()}
              >
                {modelsQuery.isFetching ? (
                  <LoaderCircle className="size-4 animate-spin" />
                ) : (
                  "Load models"
                )}
              </Button>
              <Button
                type="button"
                disabled={!activeBaseURL.trim() || !model.trim() || isBusy}
                onClick={() => void handleValidate()}
              >
                {validate.isFetching ? (
                  <LoaderCircle className="size-4 animate-spin" />
                ) : (
                  <Check className="size-4" />
                )}
                Validate
              </Button>
            </div>
          </div>

          <Input
            aria-label="Custom model name"
            className="font-mono"
            value={model}
            onChange={(event) => setModel(event.target.value)}
            onBlur={(event) => {
              const nextModel = event.target.value.trim();
              if (nextModel) {
                void persistModel(nextModel);
              }
            }}
            placeholder="Or type a model name"
          />

          {isSavedEndpoint ? (
            <p className="text-sm text-muted-foreground">
              Saved endpoint: {savedBaseURL.replace(/^https?:\/\//, "")}
              {isSavedModel && savedModel ? ` · model: ${savedModel}` : null}
            </p>
          ) : null}

          {classification ? (
            <div
              className={`flex gap-3 rounded-lg border px-3 py-3 text-sm ${
                classification.local
                  ? "border-emerald-500/30 bg-emerald-500/5"
                  : "border-amber-500/30 bg-amber-500/5"
              }`}
            >
              {classification.local ? (
                <ShieldCheck className="mt-0.5 size-4 shrink-0 text-emerald-600" />
              ) : (
                <ShieldAlert className="mt-0.5 size-4 shrink-0 text-amber-600" />
              )}
              <div>
                <p className="font-medium">
                  {classification.local
                    ? "Private — on-device"
                    : "Cloud — data may leave your device"}
                </p>
                <p className="mt-1 text-muted-foreground">
                  {classification.verdict}
                </p>
              </div>
            </div>
          ) : null}

          {validationMessage ? (
            <p className="text-sm text-muted-foreground">{validationMessage}</p>
          ) : null}
        </div>
      </SettingBlock>
    </div>
  );
}
