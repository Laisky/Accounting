# 用户侧 UI/UX 整改手册

- 状态:待开发组扩充为可交付执行的开发手册(本文只定方向、布局与验收口径,不含实现细节)
- 日期:2026-07-06
- 产品阶段:未发布。允许破坏性重构;**禁止任何向后兼容垫层**(不保留旧 class、旧主题机制、旧调色板的别名或双轨并存),一律干净切换。
- 范围:`web/` 全部用户可见界面 —— 公开页(Landing/Boarding、Auth 全流程)与认证后工作台全部视图(Home、Accounts、Account Transactions、Record、Reports、Imports、Me/Security、Search、Entry Detail)及应用壳(导航、头部、通知)。
- 明确不在范围:后端 API 与领域模型、i18n 架构(仅允许新增/修改文案 key)、React/Vite 技术栈更换、引入第三方组件库或图表库(**继续手写组件与 SVG 图表**)。
- 关联文档:`docs/proposals/2026-07-06-architecture-overhaul.md`(工程架构,其 Phase 3/4 与本手册的 W1/W4 工作流对应,由开发组合并排期)、`docs/arch/frontend.md`(2026 UI/CSS 基线)、`docs/arch/i18n.md`。

## 1. 背景

### 1.1 问题陈述

当前产品工程栈现代(React 19、OKLCH、容器查询、TanStack Query),但**视觉与交互层停留在"能用"阶段,整体观感陈旧**。基于 2026-07-06 代码树的诊断:

| ID | 发现 | 证据 | 用户感知 |
| --- | --- | --- | --- |
| U1 | 无色彩/字体设计令牌,~600 处裸 `oklch()` 字面量散落在 21 个 CSS 文件 | `src/styles.css`、`features/**/**.css`;`styles/tokens.css` 仅含布局变量 | 页面间色彩细微不一致,整体"东拼西凑"感 |
| U2 | 暗色模式双轨且残缺:class 切换(`.mobileShellThemeDark`)只覆盖 3 个 CSS 文件,`mobile-navigation.css` 又用 `prefers-color-scheme`;Landing/Auth 完全无暗色 | `MobileWorkspace.tsx:589`、`mobile-navigation.css:223`、`ThemeContext.tsx` 未设置 `color-scheme` | 切换暗色后大面积白块、导航与内容主题错位 |
| U3 | 报表调色板为硬编码 hex(`#2ec4b6` 等),与全局 OKLCH 双语言并存;图表色内联在 style 属性 | `reports/reportColors.ts`、`ReportVisuals.tsx:197,235,244` | 报表配色与全局气质割裂 |
| U4 | 字重滥用:650–950 共十余档,标题普遍 800–930 | `styles.css`(18 处 ≥800)、`landing.css`(950/900/850)、`mobile-shell.css` | 厚重、拥挤、"上个时代的加粗美学" |
| U5 | 桌面端本质是"手机壳放大":430px 圆角 `.phoneFrame` 铺满窗口,≥1024px 才出现侧栏,信息密度低 | `mobile-shell.css`、`docs/arch/frontend.md` | 桌面用户看到的是被拉宽的手机 App,不像专业记账工作台 |
| U6 | Landing 页为 div 拼装的静态假截图 + 传统分节营销页,无动效、无差异化记忆点 | `features/landing/LandingPage.tsx`、`landing.css` | 第一印象平庸,转化说服力弱 |
| U7 | 无共享 UI 基元:按钮/输入/下拉/弹层/卡片各 feature 自造,焦点环、触控尺寸、危险操作确认不统一 | `src/components/` 仅 3 个通用组件 | 同一动作在不同页面长得不一样、手感不一样 |
| U8 | 三态(空/加载/错误)存在但不统一:Suspense 兜底是一行纯文本,空态无插画/无引导动作 | 各 view 的 `emptyState` class、`MobileWorkspaceContent.tsx` | 新用户首屏冷冰冰,不知道下一步做什么 |
| U9 | 动效几乎缺席,仅零星 transition;无统一时长/缓动/减弱动效策略 | 全部 CSS | 交互反馈生硬,"没有生命感" |
| U10 | 图表交互不齐:AccountTransactions 迷你图已有 scrub/tooltip,报表 donut/bar/trend 交互与无障碍程度参差 | `AccountTransactionsView.tsx`、`ReportVisuals.tsx` | 图表"看得见摸不着",移动端尤甚 |

### 1.2 整改目标

把产品从"功能可用"提升到 **2026 年主流 SaaS 财务工具的第一梯队观感**:

1. **轻盈、精密、可信**的统一设计语言:大留白、克制的色彩、清晰的层级、柔和的深度,数字排版精密(tabular-nums、对齐、千分位)。
2. **一套语义化令牌驱动的全量明/暗双主题**,任意页面任意组件双主题完整。
3. **桌面是真正的多栏工作台**,不是放大的手机;手机保持单手可达的底部导航 + 大拇指热区。
4. **每一个用户可见图表都达到交互标准**(指针+触控 scrub、tooltip、键盘、ARIA),延续现有手写 SVG 路线。
5. **WCAG 2.2 AA** 为发布底线;动效尊重 `prefers-reduced-motion`。
6. Landing/Boarding 页具备**产品级第一印象**:真实感的动态产品演示、清晰的价值主张、双主题、五语言不破版。

### 1.3 设计原则(开发组扩充时不得违背)

1. 令牌先行:任何颜色、字号、圆角、阴影、动效时长必须来自令牌;裸字面量视为缺陷。
2. 密度自适应而非功能裁剪:手机端不得隐藏记账能力(沿袭 `frontend.md` 既有铁律)。
3. 组件基元只做当前流程需要的,不造通用组件库。
4. 动效服务于因果理解(哪里来、去哪里、发生了什么),不做装饰性动画。
5. 金额与破坏性操作必须可撤销、可确认、可核对(呼应 WCAG 2.2 错误预防条款)。
6. 干净切换:新旧样式机制不并存,迁移到哪个页面就删掉哪个页面的旧样式。

## 2. 设计语言规范(方向性定义,细节由开发组落地)

### 2.1 令牌体系(W1 的规范来源)

三层结构,全部置于 `styles/tokens.css`(或拆分为 `tokens/*.css` 并经 `@layer tokens` 组织):

```
primitive  →  semantic  →  component(仅必要时)
--oklch 原子色阶      --color-bg-surface        --button-primary-bg
--font-size 字阶      --color-text-primary      --nav-item-active-fg
--space 间距阶        --color-border-subtle
--radius / --shadow   --color-accent / -danger / -success
--motion-duration/easing
```

- 色彩:全量 OKLCH;中性灰 10–12 档、品牌主色 1 组、语义色(收入/支出/转账/危险/警告/成功)各 1 组、图表分类色 8–10 档(替换 `reportColors.ts` 的 hex,同源生成明暗两套)。
- 明暗主题:语义层通过 `:root[data-theme]` + `color-scheme` 一次性切换;组件 CSS 只允许引用语义层。删除 `.mobileShellThemeDark/Light` 与散落的 `prefers-color-scheme` 双轨机制。
- 字体:维持系统栈;字阶 1.2 模数(约 12/13/15/18/22/28/34);**字重全站收敛为 400/500/600/700 四档,禁止 >700**。金额一律 `tabular-nums`。
- 间距:4px 基准;圆角 4/8/12/16/full 五档;阴影 3 层(卡片/浮层/模态),低饱和低扩散。
- 动效:时长 120/200/320ms 三档 + 标准/退出两条缓动曲线;`prefers-reduced-motion: reduce` 时全部降级为透明度切换或直接跳变。

### 2.2 布局 DSL 约定

本手册用以下 DSL 描述核心页面布局。开发组扩充时沿用同一记法。

```
语法:
  page <名称> { viewport <断点>: <树> }
  容器: stack(纵) | row(横) | grid(cols:[...]) | scroll(可滚区)
  修饰: [sticky] [fixed] [collapsible] [max:<宽>] [gap:<档>]
  叶子: 组件名(参数)   ;  fab = 悬浮主操作按钮
断点(与现状对齐): phone <768 | tablet 768–1023 | desktop ≥1024
```

## 3. 变更清单

分两层:**横向工作流 W1–W6**(基础设施,先行)与**纵向页面整改 P1–P12**(依赖 W 层)。每项由开发组扩充为带文件清单的执行任务。

### W1 设计令牌与主题统一

- 重建 `styles/tokens.css` 为 2.1 的三层令牌;所有 feature CSS 改引语义令牌,清零裸 `oklch()`/hex 字面量。
- 统一主题机制为 `data-theme`(system/light/dark 三选项保留在 Me 设置),`ThemeContext` 同步设置文档级 `color-scheme`;删除双轨旧机制。
- Stylelint 新增规则:`tokens` 层之外禁止色彩字面量、禁止 `font-weight` >700(作为 CI 门禁,呼应架构手册 D10/D12)。

### W2 排版与图标

- 全站字阶/字重按 2.1 收敛;清理 650–950 档。
- 金额组件化:统一符号位置、千分位、正负色、`tabular-nums`。
- 图标继续 lucide-react,统一 2 档尺寸与描边宽度令牌。

### W3 共享组件基元(`src/components/ui/`)

只建当前流程用到的:`Button`(含 danger/loading)、`Input/Field`、`Select`、`Sheet`(移动底部抽屉/桌面侧滑或居中对话框自适应)、`Dialog`(危险操作确认)、`Card`、`EmptyState`(插画位+标题+引导动作)、`Skeleton`、`Toast`、`SegmentedControl`、`Tag`、`AmountText`。全部满足:44px 触控目标、`:focus-visible` 焦点环令牌、键盘可达、ARIA 完整。各 feature 自造的同类实现随页面整改删除。

### W4 应用壳:从"手机壳"到自适应工作台

- 删除桌面端 `.phoneFrame` 模型(≤520px 真机全出血形态保留)。
- 目标壳布局:

```
page AppShell {
  viewport phone:
    stack {
      header[sticky] { BookSwitcher SearchTrigger OverflowMenu }
      scroll { <ActiveView> }
      tabbar[fixed] { Home Accounts fab:Record Reports Me }
    }
  viewport tablet:
    grid(cols:[76px,1fr]) {
      rail { Brand NavIcons ThemeToggle }
      stack { header[sticky] scroll{ <ActiveView> } }
    }
  viewport desktop:
    grid(cols:[248px,minmax(0,1fr)]) {
      sidebar { Brand Nav BookSwitcher UserCard }
      stack { header[sticky]{ Breadcrumb Search QuickRecord }
              scroll[max:1440px] { <ActiveView 多栏形态> } }
    }
}
```

- 桌面 header 常驻"快速记一笔"入口(打开 Record 的桌面形态),弥补 FAB 在桌面消失后的主动作可达性。
- 通知条(notice 区)并入 Toast 体系,退出独立 grid area。

### W5 动效系统

- 令牌化时长/缓动;定义四类标准动效:页面切换(轻淡入+位移)、弹层(底部抽屉弹簧感/对话框缩放)、列表增删(高度+透明度)、数值变化(金额滚动)。
- 关键微交互:记账成功的确认反馈、tab 切换指示器滑动、图表 scrub 十字线跟手。

### W6 图表交互标准落地(延续既有标准)

所有用户可见图表(AccountTransactions 迷你趋势、Reports 的 donut/ranked bar/trend)统一达到:指针+触控 scrub、跟随 tooltip、键盘左右键步进+焦点态、`role="img"`+数据摘要 ARIA、双主题取色自图表令牌、`prefers-reduced-motion` 降级。抽出共享的 scrub/tooltip/坐标轴工具模块(仍是手写 SVG,不引库)。

### P1 Landing/Boarding 页

现状:静态假截图 + 五段式营销页,无暗色。目标:产品级第一印象。

```
page Landing {
  viewport phone:
    stack {
      header[sticky] { Logo row{ LanguageSelector ThemeToggle SignIn } }
      scroll {
        Hero { Headline Subline row{ CTA:注册 CTA2:查看演示 }
               LiveProductDemo(auto-play, 真实组件渲染的记账→报表微剧本) }
        ValueProps(grid:1col, 3–4 张卡)
        FeatureTour(交替图文, 桌面/手机双形态截图)
        TrustAndDataOwnership
        FinalCTA  Footer
      }
    }
  viewport desktop:
    同结构, Hero 变 grid(cols:[5fr,7fr]) 左文右演示,
    ValueProps 3col, FeatureTour 左右交替
}
```

- LiveProductDemo 用真实组件+种子数据循环演示"记一笔→分类→报表变化",替代 div 假截图;`prefers-reduced-motion` 时退化为静态首帧。
- 全页接入令牌与双主题;五语言文案长度差异不破版。

### P2 Auth 全流程(登录/TOTP/注册/验证/找回)

- 接入令牌、双主题、W3 表单基元;统一为居中单卡布局(桌面左侧加品牌氛围面板)。
- 明确步骤指示(注册验证、找回确认为多步),错误信息就近显示并可被屏幕阅读器播报;Turnstile 位置与加载占位规范化。

```
page Auth { viewport desktop:
  grid(cols:[5fr,7fr]) { BrandPanel(氛围插画+价值一句话)
                         stack[max:420px]{ StepIndicator? FormCard AltActions } }
  viewport phone: stack[max:420px]{ Logo FormCard AltActions } }
```

### P3 Home

现状:预算卡+最近流水。目标:一屏看清"我现在的钱怎么样了"。

```
page Home {
  viewport phone: stack[gap:m] {
    GreetingRow(日期+账本)
    NetSummaryCard(本月收/支/结余, 金额滚动动效, 迷你趋势)
    QuickActions(row: 记一笔/转账/导入)
    BudgetCard(进度条→环形或分段条, 超支预警色)
    RecentEntries(按日分组, EmptyState:引导记第一笔)
  }
  viewport desktop: grid(cols:[2fr,1fr]) {
    stack{ NetSummaryCard(带30日趋势图,达 W6 标准) RecentEntries }
    stack{ BudgetCard QuickActions UpcomingHints? }
  }
}
```

### P4 Accounts

- 分组账户树改为卡片化分组 + 每账户余额迷你趋势(可选);账户色点/图标令牌化。
- 建账、账本切换/改名/币种收敛进 `Sheet` 基元;桌面双栏(左树右详情)沿用既有 2-pane 但按新令牌重绘。

### P5 Account Transactions

- 已有月分组+scrub 迷你图,按 W1/W6 重绘取色与 tooltip;月份小结行(收/支/净)强化。
- 桌面形态:趋势图升为页首横幅图,列表变两栏密度。

### P6 Record(核心录入)

体验优先级最高的页面。目标:三次点击内完成一笔常规记账。

```
page Record {
  viewport phone: stack {
    TypeSwitch(segmented: 支出/收入/转账)
    AmountPad(超大金额显示, tabular-nums, 自定义数字键盘可选)
    QuickCategoryGrid(高频分类 8 宫格, 长按进管理)
    FieldRows[collapsible] { 账户 日期 成员 商家 标签 备注 }
    SubmitBar[fixed](主按钮+连续记账开关)
  }
  viewport desktop: grid(cols:[1fr,1fr]) {
    ComposerCard(同上纵排)  AsidePreview(当日已记+分类速览)
  }
}
```

- 提交成功给 200ms 级确认动效(W5),"连续记账"模式不离开页面。
- 分类管理(CategoryManager)套用 `Sheet` 基元。

### P7 Reports

- 七维度导航改 `SegmentedControl`/桌面侧内 tab;donut/bar/trend 全部达 W6 标准并换图表令牌色。
- 桌面形态:总览卡行 + 主图 + 明细列表三段式;维度间切换有轻动效衔接。

### P8 Imports

- 分阶段状态机(empty→…→applied)可视化为步骤条;映射配置与预览按桌面 2-pane 展开(解除 arch 备忘中"保守 CSS-only"的遗留);失败态给出可操作的错误列表。

### P9 Me / Security(含 Passkey、TOTP)

- 桌面多栏卡片网格保留,按令牌重绘;主题三选项(system/light/dark)入口在此;危险操作(删除 passkey、禁用 TOTP)走 `Dialog` 确认基元。

### P10 Search

- 搜索入口在 header 常驻;结果页复用 RecentEntries 列表形态与空态基元;搜索词高亮。

### P11 Entry Detail / 编辑

- 详情 hero 金额大字 + 类型色;编辑复用 Record 表单基元;删除走 `Dialog` 确认且支持撤销 Toast。

### P12 三态与引导(横切)

- 每个视图必须有:骨架加载态(Skeleton)、含引导动作的空态(EmptyState)、可重试的错误态;Suspense 兜底从纯文本换为骨架。
- 新用户首登引导:Home 空态即引导流(建第一个账户→记第一笔),不做独立 onboarding 向导页。

## 4. 实现方法(阶段与工作方式,非实现细节)

| 阶段 | 内容 | 依赖 | 出口条件 |
| --- | --- | --- | --- |
| S0 | W1 令牌 + W2 排版 + 主题机制统一;建立 Playwright 截图基线与 Stylelint 门禁 | 无(可与架构手册 Phase 4 合并) | 门禁生效;全站在新令牌下无回归 |
| S1 | W3 组件基元 + W5 动效令牌 + W6 图表共享工具 | S0 | 基元有独立 story/测试,双主题、a11y 达标 |
| S2 | W4 应用壳(去 phone-frame、三断点壳) | S1 | 三断点截图基线更新,导航 a11y 通过 |
| S3 | P1–P11 按页迁移,顺序建议:P6 Record → P3 Home → P7 Reports → P5/P4 → P1 Landing → P2 Auth → P8–P11;每页一个 PR,迁移即删除该页旧 CSS/自造组件 | S2 | 每页满足 §6 该页验收行 |
| S4 | P12 三态横扫 + 全站 polish(对齐/间距/一致性)+ 无障碍与性能审计 | S3 | §6 全矩阵绿 |

工作方式约束:

- 每个 PR 附三断点 × 双主题截图;视觉回归基线随 PR 更新并由 reviewer 核对。
- 五语言破版检查纳入每页出口(至少 zh/en/ja 三语言截图,es/fr 抽查)。
- 不设 feature flag、不留旧样式开关——每页切换是原子的。

## 5. 测试矩阵

| 维度 | 手段 | 覆盖对象 | 通过口径 |
| --- | --- | --- | --- |
| 视觉回归 | Playwright 截图对比 | P1–P11 × {phone,tablet,desktop} × {light,dark} | 与评审基线像素级一致 |
| 无障碍 | axe(Playwright 集成)+ 手动键盘走查 | 全部页面 + W3 基元 + 全部图表 | axe 0 serious/critical;纯键盘可完成登录→记账→看报表→登出 |
| 对比度 | 令牌级自动校验脚本 | 全部语义色对(双主题) | 正文 ≥4.5:1,大字/图形 ≥3:1(WCAG 2.2 AA) |
| 响应式 | 手动 + 截图,断点 375/430/768/1024/1440 | 全部页面 | 无横向滚动、无遮挡、触控目标 ≥44px |
| 主题 | 截图矩阵 + 切换即时性检查 | 全部页面含 Landing/Auth | 双主题无未适配区块;切换无闪白 |
| 图表交互 | Playwright 指针/触控/键盘脚本 | 迷你趋势、donut、ranked bar、trend | scrub/tooltip/键盘步进/ARIA 摘要全通过 |
| 动效 | 手动 + `prefers-reduced-motion` 强制用例 | W5 四类动效 | 减弱动效下无位移动画;时长/缓动来自令牌 |
| i18n | 五语言截图抽查 + `pnpm run check:i18n` | P1、P2、P6、P7 全量,其余抽查 | 无溢出、无截断、无破版;key 校验通过 |
| 性能 | Lighthouse(Landing、Home、Reports) | 冷加载 + 交互 | LCP ≤2.5s、INP ≤200ms、CLS ≤0.1(实验室口径) |
| 门禁 | Stylelint/CI | 全部 CSS | 令牌层外色彩字面量 0;`font-weight>700` 0 |
| 单元/E2E 存量 | Vitest + 既有 Playwright 流程 | 全部改动页 | 存量测试全绿(允许因 DOM 调整更新断言) |

## 6. 验收标准

整改整体验收(全部满足才算完成):

1. **令牌覆盖率 100%**:`styles/tokens.css`(或 tokens 目录)之外,`grep -R 'oklch(\|#[0-9a-fA-F]\{3,8\}'` 在 `web/src/**/*.css` 命中为 0;Stylelint 门禁在 CI 生效。
2. **双主题完整**:任意路由在 light/dark 下无未适配区块;主题机制仅剩 `data-theme` 一条通路,旧 class 机制与散落的 `prefers-color-scheme` 色彩声明全部删除;文档级 `color-scheme` 正确设置。
3. **排版收敛**:全站 `font-weight` 仅 400/500/600/700;金额处处 `tabular-nums` 且格式一致。
4. **桌面工作台**:≥1024px 不存在 `.phoneFrame` 视觉;Home/Reports/Record/Accounts 呈现本手册 DSL 描述的多栏形态;≤520px 保持全出血手机形态且底部导航可用。
5. **图表标准**:每一个用户可见图表满足 W6 六项(scrub/tooltip/键盘/ARIA/令牌取色/减弱动效降级),`reportColors.ts` 的 hex 调色板被图表令牌替代并删除。
6. **组件基元**:W3 清单全部落地且被对应页面实际使用;各 feature 内重复自造的按钮/弹层/表单实现删除;危险操作(删条目、删 passkey、禁用 TOTP、应用导入)全部经确认 Dialog 且文案明确后果。
7. **三态覆盖**:P1–P11 每视图具备骨架加载、引导性空态、可重试错误态;新用户空账本首登可沿空态引导完成"建账→记第一笔"。
8. **Landing 达标**:动态产品演示替代静态假截图;双主题、五语言、三断点全部不破版;Lighthouse 性能口径达 §5。
9. **无障碍**:§5 无障碍与对比度行全绿;记账与删除等资金相关操作满足 WCAG 2.2 错误预防(可确认/可撤销)。
10. **测试矩阵全绿**:§5 全部行通过;截图基线入库;存量 Vitest/Playwright 全绿。
11. **文档同步**:`docs/arch/frontend.md` 更新为新令牌/主题/壳布局的事实描述;本手册移入 `docs/proposals/archives`(按仓库惯例)由开发手册取代。

单页验收模板(开发组扩充每页任务时套用):双主题三断点截图通过评审;axe 无 serious+;键盘全流程可达;i18n 抽查不破版;该页旧样式与自造组件已删除;对应 DSL 布局逐区块核对一致。

## 7. 风险与决策点

| 风险 | 缓解 |
| --- | --- |
| 全站重绘期间视觉新旧混搭 | S3 按页原子切换,页内不混;发布前完成全量,产品未发布故无用户暴露 |
| 令牌一次到位设计成本高 | S0 先定 primitive+semantic 两层,component 层随 P 页按需补 |
| Landing 动态演示体积/性能 | 复用真实组件与既有种子数据,不引入视频/大图;懒加载并给 LCP 留静态首帧 |
| 截图基线维护成本 | 基线仅在页面级 PR 更新,评审人核对 diff 即评审设计 |
| 与架构手册 Phase 3(壳拆分)排期冲突 | W4 壳重构与 Phase 3 组件拆分合并为同一批 PR 执行,避免同文件两次大改 |
