import {
  Check,
  LoaderCircle,
  RefreshCw,
  ShieldCheck,
  ShieldAlert,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldDescription,
  FieldLabel,
  FieldTitle,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
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

  const [baseURLDraft, setBaseURLDraft] = useState("");
  const [modelDraft, setModelDraft] = useState<string | null>(null);
  const [apiKey, setApiKey] = useState("");
  const [validationMessage, setValidationMessage] = useState<string | null>(
    null,
  );

  const discovery = useDiscoverLocalAIEndpoints();
  const activeBaseURL = baseURLDraft || savedBaseURL;
  const activeModel = modelDraft ?? savedModel;
  const classify = useClassifyAIEndpoint(activeBaseURL);
  const modelsQuery = useAIModels(activeBaseURL, apiKey);
  const validate = useValidateAIConfig(activeBaseURL, apiKey, activeModel);
  const saveEndpoint = useSaveAIEndpoint();
  const saveModel = useSaveAIModel();
  const clearModel = useClearAIModel();
  const saveConfig = useSaveAIConfig();

  const modelSavedForActiveEndpoint = Boolean(
    savedModel &&
      savedBaseURL &&
      aiEndpointsMatch(activeBaseURL, savedBaseURL),
  );

  const models = modelsQuery.data ?? [];

  useEffect(() => {
    if (savedBaseURL) {
      setBaseURLDraft(savedBaseURL);
    }
  }, [savedBaseURL]);

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
    () => Boolean(savedModel && activeModel === savedModel && modelSavedForActiveEndpoint),
    [activeModel, modelSavedForActiveEndpoint, savedModel],
  );

  const refreshModels = async () => {
    const result = await modelsQuery.refetch();
    const nextModels = result.data ?? [];
    if (
      savedModel &&
      modelSavedForActiveEndpoint &&
      nextModels.length > 0 &&
      !nextModels.includes(savedModel)
    ) {
      await clearModel.mutateAsync();
      setModelDraft(null);
    }
  };

  const handleSelectEndpoint = async (endpoint: AIEndpoint) => {
    setValidationMessage(null);
    setBaseURLDraft(endpoint.baseUrl);
    setModelDraft(null);
    await saveEndpoint.mutateAsync(endpoint.baseUrl);

    const switchingEndpoint = !aiEndpointsMatch(endpoint.baseUrl, savedBaseURL);
    if (switchingEndpoint) {
      await clearModel.mutateAsync();
    }

    if (!endpoint.running) {
      setValidationMessage(
        `${endpoint.name} is not running. Start it and scan again to load models.`,
      );
    }
  };

  const handleModelChange = (nextModel: string) => {
    setModelDraft(null);
    void saveModel.mutate(nextModel);
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
      await saveConfig.mutateAsync({ baseURL: activeBaseURL, model: activeModel });
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
        <Field>
          <div className="flex items-center justify-between gap-3">
            <FieldTitle>Detected endpoints</FieldTitle>
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
        </Field>
      </SettingBlock>

      <SettingBlock
        title="Connection"
        description="Configure the OpenAI-compatible base URL and model Clockr should use."
      >
        <Field>
            <FieldLabel htmlFor="ai-base-url">Base URL</FieldLabel>
            <Input
              id="ai-base-url"
              className="font-mono"
              value={baseURLDraft}
              onChange={(event) => setBaseURLDraft(event.target.value)}
              onBlur={() => {
                const trimmed = baseURLDraft.trim();
                if (trimmed) {
                  void saveEndpoint.mutate(trimmed);
                }
              }}
              placeholder="http://127.0.0.1:11434/v1"
            />
          </Field>

          <Field>
            <FieldLabel htmlFor="ai-api-key">
              API key{" "}
              <span className="font-normal text-muted-foreground">
                (optional)
              </span>
            </FieldLabel>
            <Input
              id="ai-api-key"
              type="password"
              className="font-mono"
              value={apiKey}
              onChange={(event) => setApiKey(event.target.value)}
              placeholder="Not required for local models"
            />
          </Field>

          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field>
              <FieldLabel htmlFor="ai-model">Model</FieldLabel>
              <Select
                value={activeModel || undefined}
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
            </Field>

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
                disabled={!activeBaseURL.trim() || !activeModel.trim() || isBusy}
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

          <Field>
            <FieldLabel htmlFor="ai-custom-model" className="sr-only">
              Custom model name
            </FieldLabel>
            <Input
              id="ai-custom-model"
              className="font-mono"
              value={activeModel}
              onChange={(event) => setModelDraft(event.target.value)}
              onBlur={(event) => {
                const nextModel = event.target.value.trim();
                setModelDraft(null);
                if (nextModel) {
                  void saveModel.mutate(nextModel);
                }
              }}
              placeholder="Or type a model name"
            />
          </Field>

          {isSavedEndpoint ? (
            <FieldDescription>
              Saved endpoint: {savedBaseURL.replace(/^https?:\/\//, "")}
              {isSavedModel && savedModel ? ` · model: ${savedModel}` : null}
            </FieldDescription>
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
            <FieldDescription>{validationMessage}</FieldDescription>
          ) : null}
      </SettingBlock>
    </div>
  );
}
