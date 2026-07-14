export const CHANNEL_TYPE_OPTIONS = [
  { value: 1, labelKey: "typeOpenAI" },
  { value: 2, labelKey: "typeAnthropic" },
  { value: 3, labelKey: "typeGemini" },
  { value: 4, labelKey: "typeVolcengine" },
  { value: 5, labelKey: "typeOpenAIEmbedding" },
  { value: 6, labelKey: "typeMiMoChat" },
  { value: 7, labelKey: "typeCloudflare" },
] as const;

// DEFAULT_CHANNEL_TYPE 与渠道类型下拉选项的首项保持一致（issue #143）。
// 新建渠道表单初始化和重置时使用此值，确保默认类型与前端展示逻辑同步--
// 调整 CHANNEL_TYPE_OPTIONS 顺序时默认值自动跟随，无需多处修改。
export const DEFAULT_CHANNEL_TYPE = CHANNEL_TYPE_OPTIONS[0].value;
