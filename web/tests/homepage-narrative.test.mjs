import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const homePath = new URL(
  "../classic/src/pages/Home/index.jsx",
  import.meta.url,
);
const enLocalePath = new URL(
  "../classic/src/i18n/locales/en.json",
  import.meta.url,
);
const zhCNLocalePath = new URL(
  "../classic/src/i18n/locales/zh-CN.json",
  import.meta.url,
);
const zhTWLocalePath = new URL(
  "../classic/src/i18n/locales/zh-TW.json",
  import.meta.url,
);
const frLocalePath = new URL(
  "../classic/src/i18n/locales/fr.json",
  import.meta.url,
);
const jaLocalePath = new URL(
  "../classic/src/i18n/locales/ja.json",
  import.meta.url,
);
const ruLocalePath = new URL(
  "../classic/src/i18n/locales/ru.json",
  import.meta.url,
);
const viLocalePath = new URL(
  "../classic/src/i18n/locales/vi.json",
  import.meta.url,
);

const load = async (path) => readFile(path, "utf8");
const homepageNarrativeExpectations = {
  "zh-CN": {
    "Agent 能力流动性基础设施": "Agent 能力流动性基础设施",
    "面向 Agent 能力流动性的基础设施": "面向 Agent 能力流动性的基础设施",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。",
    能力流动性: "能力流动性",
    供需网络: "供需网络",
    执行秩序: "执行秩序",
    可信结算: "可信结算",
    供给抽象: "供给抽象",
    将异构智能能力抽象为流动性层: "将异构智能能力抽象为流动性层",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。",
    流动网络: "流动网络",
    "支撑 Agent 的连续能力获取": "支撑 Agent 的连续能力获取",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。",
    执行秩序: "执行秩序",
    让能力流动具备确定性: "让能力流动具备确定性",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。",
  },
  "zh-TW": {
    "Agent 能力流动性基础设施": "Agent 能力流動性基礎設施",
    "面向 Agent 能力流动性的基础设施": "面向 Agent 能力流動性的基礎設施",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken 將異構智能能力抽象為可發現、可組合、可調度、可結算的流動性層。",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "讓 Agent 在統一供需網絡中獲得連續、可信、可審計的能力供給。",
    能力流动性: "能力流動性",
    供需网络: "供需網絡",
    执行秩序: "執行秩序",
    可信结算: "可信結算",
    供给抽象: "供給抽象",
    将异构智能能力抽象为流动性层: "將異構智能能力抽象為流動性層",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "不同來源的智能能力被統一表達為可供給、可組合、可約束、可履約的能力單元。",
    流动网络: "流動網絡",
    "支撑 Agent 的连续能力获取": "支撐 Agent 的連續能力獲取",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agent 不再綁定靜態資源，而是在統一供需網絡中按場景獲得持續、穩定的能力供給。",
    执行秩序: "執行秩序",
    让能力流动具备确定性: "讓能力流動具備確定性",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "發現、調度、執行、結算與審計被納入同一秩序，支撐能力在多主體之間規模化流動。",
  },
  en: {
    "Agent 能力流动性基础设施": "Infrastructure for Agent capability liquidity",
    "面向 Agent 能力流动性的基础设施":
      "Infrastructure for Agent capability liquidity",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken abstracts heterogeneous intelligent capabilities into a discoverable, composable, schedulable, and settleable liquidity layer.",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "Give Agents continuous, trustworthy, auditable capability supply within a unified supply-demand network.",
    能力流动性: "Capability Liquidity",
    供需网络: "Supply-Demand Network",
    执行秩序: "Execution Order",
    可信结算: "Trusted Settlement",
    供给抽象: "Supply Abstraction",
    将异构智能能力抽象为流动性层:
      "Abstract heterogeneous intelligent capabilities into a liquidity layer",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "Capabilities from different sources are unified into units that can be supplied, composed, constrained, and fulfilled.",
    流动网络: "Liquidity Network",
    "支撑 Agent 的连续能力获取":
      "Support continuous capability acquisition for Agents",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agents are no longer bound to static resources; they obtain sustained, stable capability supply within a unified supply-demand network.",
    执行秩序: "Execution Order",
    让能力流动具备确定性: "Give capability flow determinism",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "Discovery, scheduling, execution, settlement, and auditing are brought into one order to scale capability flow across multiple parties.",
  },
  fr: {
    "Agent 能力流动性基础设施":
      "Infrastructure de liquidité des capacités Agent",
    "面向 Agent 能力流动性的基础设施":
      "Infrastructure pour la liquidité des capacités Agent",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken abstrait des capacités intelligentes hétérogènes en une couche de liquidité découvrable, composable, planifiable et réglable.",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "Donner aux Agents une offre de capacités continue, fiable et auditée dans un réseau unifié d’offre et de demande.",
    能力流动性: "Liquidité des capacités",
    供需网络: "Réseau offre-demande",
    执行秩序: "Ordre d’exécution",
    可信结算: "Règlement fiable",
    供给抽象: "Abstraction de l’offre",
    将异构智能能力抽象为流动性层:
      "Abstraire des capacités intelligentes hétérogènes en couche de liquidité",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "Des capacités de sources différentes sont unifiées en unités pouvant être fournies, composées, contraintes et exécutées.",
    流动网络: "Réseau de liquidité",
    "支撑 Agent 的连续能力获取":
      "Soutenir l’acquisition continue de capacités pour les Agents",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Les Agents ne dépendent plus de ressources statiques; ils obtiennent une offre de capacités continue et stable dans un réseau unifié d’offre et de demande.",
    执行秩序: "Ordre d’exécution",
    让能力流动具备确定性:
      "Donner de la détermination à la circulation des capacités",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "Découverte, planification, exécution, règlement et audit sont intégrés dans un même ordre pour faire passer les capacités à l’échelle entre plusieurs acteurs.",
  },
  ja: {
    "Agent 能力流动性基础设施": "Agent能力流動性インフラ",
    "面向 Agent 能力流动性的基础设施": "Agent能力流動性のためのインフラ",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesTokenは、異なる種類の知的能力を、発見・組成・スケジューリング・決済可能な流動性レイヤーとして抽象化します。",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "Agent が統一された需給ネットワークの中で、継続的で信頼でき、監査可能な能力供給を得られるようにします。",
    能力流动性: "能力流動性",
    供需网络: "需給ネットワーク",
    执行秩序: "実行秩序",
    可信结算: "信頼できる決済",
    供给抽象: "供給の抽象化",
    将异构智能能力抽象为流动性层:
      "異なる種類の知的能力を流動性レイヤーとして抽象化する",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "異なるソースの知的能力は、供給・組成・制約・履行が可能な能力単位として統一的に表現されます。",
    流动网络: "流動ネットワーク",
    "支撑 Agent 的连续能力获取": "Agent の継続的な能力取得を支える",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agent は静的な資源に縛られず、統一された需給ネットワークの中で場面に応じて継続的で安定した能力供給を受けます。",
    执行秩序: "実行秩序",
    让能力流动具备确定性: "能力の流れに確実性を与える",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "発見、スケジューリング、実行、決済、監査を同一の秩序に収め、複数主体間での能力の流れを大規模化します。",
  },
  ru: {
    "Agent 能力流动性基础设施": "Инфраструктура ликвидности возможностей Agent",
    "面向 Agent 能力流动性的基础设施":
      "Инфраструктура для ликвидности возможностей Agent",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken абстрагирует разнородные интеллектуальные возможности в слой ликвидности, который можно обнаруживать, составлять, планировать и рассчитывать.",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "Дать Agent непрерывное, доверяемое и аудируемое предложение возможностей в единой сети спроса и предложения.",
    能力流动性: "Ликвидность возможностей",
    供需网络: "Сеть спроса и предложения",
    执行秩序: "Порядок исполнения",
    可信结算: "Надежные расчеты",
    供给抽象: "Абстракция предложения",
    将异构智能能力抽象为流动性层:
      "Абстрагировать разнородные интеллектуальные возможности в слой ликвидности",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "Возможности из разных источников объединяются в единицы, которые можно поставлять, составлять, ограничивать и исполнять.",
    流动网络: "Сеть ликвидности",
    "支撑 Agent 的连续能力获取":
      "Поддерживать непрерывное получение возможностей для Agent",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agent больше не привязан к статическим ресурсам; он получает устойчивое предложение возможностей по сценарию в единой сети спроса и предложения.",
    执行秩序: "Порядок исполнения",
    让能力流动具备确定性: "Сделать поток возможностей определенным",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "Обнаружение, планирование, исполнение, расчеты и аудит объединяются в единый порядок, обеспечивая масштабируемый поток возможностей между несколькими участниками.",
  },
  vi: {
    "Agent 能力流动性基础设施": "Hạ tầng thanh khoản năng lực Agent",
    "面向 Agent 能力流动性的基础设施": "Hạ tầng cho thanh khoản năng lực Agent",
    "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。":
      "HermesToken trừu tượng hóa các năng lực thông minh dị thể thành một lớp thanh khoản có thể được khám phá, kết hợp, lập lịch và quyết toán.",
    "让 Agent 在统一供需网络中获得连续、可信、可审计的能力供给。":
      "Để Agent có được nguồn cung năng lực liên tục, đáng tin cậy và có thể kiểm toán trong một mạng cung-cầu thống nhất.",
    能力流动性: "Thanh khoản năng lực",
    供需网络: "Mạng cung-cầu",
    执行秩序: "Trật tự thực thi",
    可信结算: "Quyết toán đáng tin",
    供给抽象: "Trừu tượng hóa cung",
    将异构智能能力抽象为流动性层:
      "Trừu tượng hóa các năng lực thông minh dị thể thành lớp thanh khoản",
    "不同来源的智能能力被统一表达为可供给、可组合、可约束、可履约的能力单元。":
      "Các năng lực thông minh từ nhiều nguồn được hợp nhất thành đơn vị có thể cung cấp, kết hợp, ràng buộc và thực thi.",
    流动网络: "Mạng thanh khoản",
    "支撑 Agent 的连续能力获取": "Hỗ trợ Agent tiếp nhận năng lực liên tục",
    "Agent 不再绑定静态资源，而是在统一供需网络中按场景获得持续、稳定的能力供给。":
      "Agent không còn bị buộc vào tài nguyên tĩnh; nó nhận nguồn cung năng lực ổn định và liên tục theo ngữ cảnh trong một mạng cung-cầu thống nhất.",
    执行秩序: "Trật tự thực thi",
    让能力流动具备确定性: "Làm cho dòng chảy năng lực có tính xác định",
    "发现、调度、执行、结算与审计被纳入同一秩序，支撑能力在多主体之间规模化流动。":
      "Khám phá, điều phối, thực thi, quyết toán và kiểm toán được đưa vào cùng một trật tự, hỗ trợ dòng chảy năng lực ở quy mô lớn giữa nhiều chủ thể.",
  },
};

test("default homepage source uses the minimal narrative layout", async () => {
  const source = await load(homePath);

  assert.ok(
    source.includes(
      "HermesToken 将异构智能能力抽象为可发现、可组合、可调度、可结算的流动性层。",
    ),
  );
  assert.ok(source.includes("Agent 能力流动性基础设施"));
  assert.ok(source.includes("能力流动性"));
  assert.ok(source.includes("供需网络"));
  assert.ok(source.includes("将异构智能能力抽象为流动性层"));
  assert.ok(source.includes("让能力流动具备确定性"));

  assert.ok(!source.includes("API Key"));
  assert.ok(!source.includes("手续费"));
  assert.ok(!source.includes("买断额度"));
  assert.ok(!source.includes("大模型接口网关"));
  assert.ok(!source.includes("text-semi-color"));
  assert.ok(!source.includes("border-semi-color"));
  assert.ok(!source.includes("bg-semi-color"));
  assert.ok(
    !source.includes("更好的价格，更好的稳定性，只需要将模型基址替换为："),
  );
  assert.ok(!source.includes("获取密钥"));
  assert.ok(!source.includes("支持众多的大模型供应商"));
});

test("homepage narrative translations exist for all supported homepage locales", async () => {
  const [
    enSource,
    zhCNSource,
    zhTWSource,
    frSource,
    jaSource,
    ruSource,
    viSource,
  ] = await Promise.all([
    load(enLocalePath),
    load(zhCNLocalePath),
    load(zhTWLocalePath),
    load(frLocalePath),
    load(jaLocalePath),
    load(ruLocalePath),
    load(viLocalePath),
  ]);

  const locales = {
    en: JSON.parse(enSource).translation,
    "zh-CN": JSON.parse(zhCNSource).translation,
    "zh-TW": JSON.parse(zhTWSource).translation,
    fr: JSON.parse(frSource).translation,
    ja: JSON.parse(jaSource).translation,
    ru: JSON.parse(ruSource).translation,
    vi: JSON.parse(viSource).translation,
  };

  for (const [locale, expectations] of Object.entries(
    homepageNarrativeExpectations,
  )) {
    for (const [key, expectedValue] of Object.entries(expectations)) {
      assert.equal(locales[locale][key], expectedValue);
    }
  }
});
