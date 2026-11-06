<script setup lang="ts">
import { computed, ref } from 'vue'

type HighlightVariant = 'edge' | 'wash' | 'rail'

interface VariantDefinition {
  key: HighlightVariant
  label: string
  title: string
  description: string
  usage: string
  color: string
}

const variants: VariantDefinition[] = [
  {
    key: 'edge',
    label: '方案 A · 强边缘',
    title: '高对比边缘',
    description: '用亮色描边和柔和外发光，把待配置卡片从列表中直接拎出来。',
    usage: '适合需要快速找到待处理项的场景。',
    color: '#e69f00',
  },
  {
    key: 'wash',
    label: '方案 B · 状态底板',
    title: '浅色状态底板',
    description: '保留卡片结构，增加淡黄色状态底板和清晰的待配置提示。',
    usage: '适合希望提示明显、但不想有强烈描边的场景。',
    color: '#d55e00',
  },
  {
    key: 'rail',
    label: '方案 C · 聚焦轨道',
    title: '左侧聚焦轨道',
    description: '用一条连续的颜色轨道和小型角标表达状态，正文保持最安静。',
    usage: '适合卡片较多、需要维持密度的配置侧栏。',
    color: '#0072b2',
  },
]

const selectedVariant = ref<HighlightVariant>('edge')
const selectedDefinition = computed(() => variants.find(({ key }) => key === selectedVariant.value))

function chooseVariant(key: HighlightVariant): void {
  selectedVariant.value = key
}
</script>

<template>
  <main class="highlight-preview">
    <header class="highlight-preview__header">
      <div>
        <p class="highlight-preview__eyebrow">趋势看板 · 视觉预览</p>
        <h1>未配置曲线的高亮方案</h1>
        <p class="highlight-preview__lede">
          三种方案都只表达同一件事：这张曲线卡还需要完成条件设置。选择一个更符合你的阅读习惯的方案。
        </p>
      </div>
      <div class="highlight-preview__selection" aria-live="polite">
        <span>当前查看</span>
        <strong>{{ selectedDefinition?.title }}</strong>
      </div>
    </header>

    <section class="highlight-preview__grid" aria-label="高亮方案预览">
      <article
        v-for="variant in variants"
        :key="variant.key"
        :class="[
          'highlight-preview__option',
          `highlight-preview__option--${variant.key}`,
          { 'highlight-preview__option--selected': selectedVariant === variant.key },
        ]"
        :style="{ '--preview-color': variant.color }"
      >
        <header class="highlight-preview__option-header">
          <div>
            <p>{{ variant.label }}</p>
            <h2>{{ variant.title }}</h2>
          </div>
          <span class="highlight-preview__option-index">{{ String(variants.indexOf(variant) + 1).padStart(2, '0') }}</span>
        </header>

        <div class="highlight-preview__card-wrap">
          <section class="highlight-preview__curve-card" aria-label="待配置曲线示例">
            <div v-if="variant.key === 'rail'" class="highlight-preview__rail-marker" aria-hidden="true"></div>
            <header class="highlight-preview__curve-header">
              <div class="highlight-preview__curve-heading">
                <span class="highlight-preview__curve-kicker">技术关键词</span>
                <strong>待配置曲线</strong>
              </div>
              <span class="highlight-preview__pending">待配置</span>
            </header>

            <div class="highlight-preview__field-grid">
              <div class="highlight-preview__field">
                <span>统计维度</span>
                <div>技术关键词</div>
              </div>
              <div class="highlight-preview__field">
                <span>统计条件</span>
                <div class="highlight-preview__placeholder">选择技术关键词</div>
              </div>
            </div>

            <div v-if="variant.key === 'rail'" class="highlight-preview__rail-note">
              <span class="highlight-preview__rail-dot"></span>
              需要完成条件
            </div>
            <div v-else class="highlight-preview__card-note">
              <span class="highlight-preview__status-icon" aria-hidden="true">!</span>
              <span>需要完成条件后才能应用</span>
            </div>
          </section>
        </div>

        <p class="highlight-preview__description">{{ variant.description }}</p>
        <p class="highlight-preview__usage">{{ variant.usage }}</p>
        <button
          type="button"
          class="highlight-preview__choose"
          :aria-pressed="selectedVariant === variant.key"
          @click="chooseVariant(variant.key)"
        >
          {{ selectedVariant === variant.key ? '正在查看' : '查看此方案' }}
        </button>
      </article>
    </section>
  </main>
</template>

<style scoped>
.highlight-preview {
  --preview-ink: #12313c;
  --preview-muted: #647b83;
  --preview-rule: #d5e5e9;
  --preview-surface: rgba(255, 255, 255, 0.9);
  min-height: 100vh;
  padding: 54px 48px 64px;
}

.highlight-preview__header {
  align-items: flex-end;
  display: flex;
  gap: 32px;
  justify-content: space-between;
  margin: 0 auto 30px;
  max-width: 1240px;
}

.highlight-preview__eyebrow {
  color: #008287;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.13em;
  margin: 0 0 8px;
}

.highlight-preview h1 {
  color: var(--preview-ink);
  font-size: clamp(1.7rem, 3vw, 2.35rem);
  letter-spacing: -0.05em;
  line-height: 1.1;
  margin: 0;
}

.highlight-preview__lede {
  color: var(--preview-muted);
  font-size: 0.9rem;
  line-height: 1.7;
  margin: 12px 0 0;
  max-width: 680px;
}

.highlight-preview__selection {
  border-left: 3px solid #0072b2;
  display: grid;
  gap: 4px;
  min-width: 160px;
  padding: 3px 0 3px 14px;
}

.highlight-preview__selection span {
  color: var(--preview-muted);
  font-size: 0.7rem;
  font-weight: 700;
}

.highlight-preview__selection strong {
  color: var(--preview-ink);
  font-size: 0.95rem;
}

.highlight-preview__grid {
  display: grid;
  gap: 18px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0 auto;
  max-width: 1240px;
}

.highlight-preview__option {
  background: var(--preview-surface);
  border: 1px solid var(--preview-rule);
  border-radius: 16px;
  box-shadow: 0 18px 42px -34px rgba(24, 77, 94, 0.72);
  display: flex;
  flex-direction: column;
  min-width: 0;
  padding: 18px;
  transition: border-color 180ms ease, box-shadow 180ms ease, transform 180ms ease;
}

.highlight-preview__option--selected {
  border-color: color-mix(in srgb, var(--preview-color), #ffffff 28%);
  box-shadow: 0 18px 42px -32px color-mix(in srgb, var(--preview-color), transparent 38%);
  transform: translateY(-3px);
}

.highlight-preview__option-header {
  align-items: flex-start;
  display: flex;
  justify-content: space-between;
  min-height: 62px;
}

.highlight-preview__option-header p {
  color: var(--preview-color);
  font-size: 0.66rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  margin: 0 0 5px;
}

.highlight-preview__option-header h2 {
  color: var(--preview-ink);
  font-size: 1.03rem;
  letter-spacing: -0.03em;
  margin: 0;
}

.highlight-preview__option-index {
  color: color-mix(in srgb, var(--preview-color), #ffffff 12%);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.72rem;
  font-weight: 800;
}

.highlight-preview__card-wrap {
  margin: 8px 0 16px;
  min-height: 178px;
}

.highlight-preview__curve-card {
  background: #ffffff;
  border: 1px solid #d8e6e9;
  border-radius: 12px;
  box-shadow: 0 11px 25px -24px rgba(25, 71, 85, 0.9);
  min-height: 178px;
  overflow: hidden;
  padding: 14px;
  position: relative;
}

.highlight-preview__option--edge .highlight-preview__curve-card {
  border: 2px solid var(--preview-color);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--preview-color), transparent 80%), 0 16px 30px -26px color-mix(in srgb, var(--preview-color), transparent 40%);
}

.highlight-preview__option--wash .highlight-preview__curve-card {
  background: color-mix(in srgb, var(--preview-color), #ffffff 90%);
  border-top: 5px solid var(--preview-color);
}

.highlight-preview__option--rail .highlight-preview__curve-card {
  padding-left: 23px;
}

.highlight-preview__rail-marker {
  background: var(--preview-color);
  bottom: 0;
  left: 0;
  position: absolute;
  top: 0;
  width: 7px;
}

.highlight-preview__curve-header {
  align-items: flex-start;
  display: flex;
  gap: 8px;
  justify-content: space-between;
}

.highlight-preview__curve-heading {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.highlight-preview__curve-kicker {
  color: var(--preview-color);
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.06em;
}

.highlight-preview__curve-heading strong {
  color: var(--preview-ink);
  font-size: 0.98rem;
  letter-spacing: -0.02em;
}

.highlight-preview__pending {
  background: color-mix(in srgb, var(--preview-color), #ffffff 82%);
  border-radius: 999px;
  color: color-mix(in srgb, var(--preview-color), #102e38 35%);
  flex: none;
  font-size: 0.64rem;
  font-weight: 800;
  padding: 4px 7px;
}

.highlight-preview__field-grid {
  border-bottom: 1px solid #e4edef;
  display: grid;
  gap: 8px;
  grid-template-columns: 0.82fr 1.18fr;
  margin-top: 16px;
  padding-bottom: 12px;
}

.highlight-preview__field {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.highlight-preview__field > span {
  color: #73868d;
  font-size: 0.62rem;
  font-weight: 700;
}

.highlight-preview__field > div {
  background: #f6fafb;
  border: 1px solid #d9e7ea;
  border-radius: 7px;
  color: #284852;
  font-size: 0.72rem;
  min-height: 28px;
  overflow: hidden;
  padding: 6px 8px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.highlight-preview__field > .highlight-preview__placeholder {
  color: #9aabb0;
}

.highlight-preview__card-note,
.highlight-preview__rail-note {
  align-items: center;
  color: #687d84;
  display: flex;
  font-size: 0.67rem;
  gap: 6px;
  margin-top: 12px;
}

.highlight-preview__status-icon {
  align-items: center;
  background: var(--preview-color);
  border-radius: 50%;
  color: #ffffff;
  display: inline-flex;
  font-size: 0.62rem;
  font-weight: 900;
  height: 16px;
  justify-content: center;
  width: 16px;
}

.highlight-preview__rail-dot {
  background: var(--preview-color);
  border-radius: 50%;
  height: 8px;
  width: 8px;
}

.highlight-preview__description,
.highlight-preview__usage {
  color: var(--preview-muted);
  font-size: 0.78rem;
  line-height: 1.6;
  margin: 0;
}

.highlight-preview__usage {
  color: #81939a;
  font-size: 0.71rem;
  margin-top: 5px;
}

.highlight-preview__choose {
  align-self: flex-start;
  background: transparent;
  border: 1px solid color-mix(in srgb, var(--preview-color), #ffffff 38%);
  border-radius: 7px;
  color: color-mix(in srgb, var(--preview-color), #102e38 25%);
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 800;
  margin-top: 16px;
  padding: 7px 10px;
  transition: background 160ms ease, color 160ms ease;
}

.highlight-preview__choose:hover,
.highlight-preview__choose:focus-visible {
  background: var(--preview-color);
  color: #ffffff;
  outline: none;
}

:global(.app-shell[data-color-scheme='dark'] .highlight-preview) {
  --preview-ink: #edf7f8;
  --preview-muted: #a8c3ca;
  --preview-rule: #2b4c58;
  --preview-surface: rgba(18, 46, 57, 0.92);
}

:global(.app-shell[data-color-scheme='dark'] .highlight-preview__curve-card) {
  background: #193640;
  border-color: #31535d;
}

:global(.app-shell[data-color-scheme='dark'] .highlight-preview__option--wash .highlight-preview__curve-card) {
  background: color-mix(in srgb, var(--preview-color), #193640 84%);
}

:global(.app-shell[data-color-scheme='dark'] .highlight-preview__field > div) {
  background: #102c35;
  border-color: #31535d;
  color: #d8ebee;
}

:global(.app-shell[data-color-scheme='dark'] .highlight-preview__field > .highlight-preview__placeholder) {
  color: #8ca9b1;
}

@media (max-width: 980px) {
  .highlight-preview {
    padding: 36px 24px 48px;
  }

  .highlight-preview__grid {
    grid-template-columns: 1fr;
    max-width: 580px;
  }

  .highlight-preview__option {
    padding: 18px 20px;
  }
}

@media (max-width: 560px) {
  .highlight-preview {
    padding: 28px 14px 40px;
  }

  .highlight-preview__header {
    align-items: flex-start;
    display: grid;
    gap: 18px;
    margin-bottom: 22px;
  }

  .highlight-preview__lede {
    font-size: 0.82rem;
  }

  .highlight-preview__selection {
    min-width: 0;
  }
}

</style>
