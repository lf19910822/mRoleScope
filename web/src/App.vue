<script setup lang="ts">
import { darkTheme, dateZhCN, NConfigProvider, useOsTheme, zhCN } from 'naive-ui'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { oceanBreezeDarkThemeOverrides, oceanBreezeThemeOverrides } from '@/theme/ocean-breeze'
import AppRequestLoader from '@/components/AppRequestLoader.vue'
import { requestActivity } from '@/lib/request-activity'

const route = useRoute()
const osTheme = useOsTheme()
const navigationOpen = ref(false)
const viewportWidth = ref(typeof window === 'undefined' ? 1440 : window.innerWidth)
const navigationExpanded = ref(defaultNavigationExpanded())
const navigationManuallyToggled = ref(false)
const lastJobListPath = ref(route.name === 'jobs' ? route.fullPath : '/jobs')
const releasedDestinations = [
  { label: '岗位筛选', name: 'jobs' as const, icon: 'jobs' },
  { label: '趋势看板', name: 'job-trends' as const, icon: 'trends' },
]
const isJobDestination = computed(() => route.name === 'jobs' || route.name === 'job-detail')
const colorScheme = computed(() => (osTheme.value === 'dark' ? 'dark' : 'light'))
const selectedTheme = computed(() => (colorScheme.value === 'dark' ? darkTheme : null))
const selectedThemeOverrides = computed(() =>
  colorScheme.value === 'dark' ? oceanBreezeDarkThemeOverrides : oceanBreezeThemeOverrides,
)
const isDesktopViewport = computed(() => viewportWidth.value > 760)

function defaultNavigationExpanded(): boolean {
  return viewportWidth.value >= 1100
}

function closeMobileNavigation(): void {
  navigationOpen.value = false
}

function toggleDesktopNavigation(): void {
  navigationManuallyToggled.value = true
  navigationExpanded.value = !navigationExpanded.value
}

function expandCollapsedNavigation(): void {
  if (!isDesktopViewport.value || navigationExpanded.value) return
  navigationManuallyToggled.value = true
  navigationExpanded.value = true
}

function handleViewportResize(): void {
  viewportWidth.value = window.innerWidth
  if (!navigationManuallyToggled.value) navigationExpanded.value = defaultNavigationExpanded()
}

watch(
  () => route.fullPath,
  () => {
    if (route.name === 'jobs') lastJobListPath.value = route.fullPath
    closeMobileNavigation()
  },
)

onMounted(() => window.addEventListener('resize', handleViewportResize))
onBeforeUnmount(() => window.removeEventListener('resize', handleViewportResize))
</script>

<template>
  <NConfigProvider
    :date-locale="dateZhCN"
    :locale="zhCN"
    :theme="selectedTheme"
    :theme-overrides="selectedThemeOverrides"
  >
    <div
      class="app-shell"
      :class="{ 'app-shell--navigation-collapsed': !navigationExpanded }"
      :data-color-scheme="colorScheme"
    >
      <button
        class="app-menu-toggle"
        type="button"
        aria-label="打开导航"
        :aria-expanded="navigationOpen"
        aria-controls="application-navigation"
        data-testid="open-navigation"
        @click="navigationOpen = true"
      >
        <span></span><span></span><span></span>
      </button>
      <div
        v-if="navigationOpen"
        class="app-navigation-overlay"
        aria-hidden="true"
        data-testid="navigation-overlay"
        @click="closeMobileNavigation"
      ></div>
      <aside
        :class="[
          'app-sidebar',
          { 'app-sidebar--expanded': navigationExpanded },
          { 'app-sidebar--collapsed': !navigationExpanded },
          { 'app-sidebar--mobile-open': navigationOpen },
        ]"
        :aria-hidden="false"
      >
        <div class="app-sidebar__header">
          <div
            class="app-sidebar__brand"
            :class="{ 'app-sidebar__brand--expand': !navigationExpanded && isDesktopViewport }"
            :role="!navigationExpanded && isDesktopViewport ? 'button' : undefined"
            :tabindex="!navigationExpanded && isDesktopViewport ? 0 : undefined"
            :aria-label="!navigationExpanded && isDesktopViewport ? '展开导航栏' : undefined"
            data-testid="navigation-brand"
            @click="expandCollapsedNavigation"
            @keydown.enter.prevent="expandCollapsedNavigation"
            @keydown.space.prevent="expandCollapsedNavigation"
          >
            <span class="app-sidebar__wave" aria-hidden="true"><i></i><i></i><i></i></span>
            <div class="app-sidebar__brand-copy">
              <strong>mRoleScope</strong>
              <small>JD OBSERVATORY</small>
            </div>
          </div>
          <button
            v-if="navigationExpanded"
            class="app-sidebar__desktop-toggle"
            type="button"
            aria-label="隐藏导航栏"
            :aria-expanded="navigationExpanded"
            aria-controls="application-navigation"
            data-testid="toggle-desktop-navigation"
            @click="toggleDesktopNavigation"
          >
            <span aria-hidden="true">‹</span>
          </button>
          <button
            class="app-sidebar__mobile-close"
            type="button"
            aria-label="关闭导航"
            data-testid="close-navigation"
            @click="closeMobileNavigation"
          >
            ×
          </button>
        </div>
        <nav id="application-navigation" aria-label="主导航" data-testid="application-navigation">
          <RouterLink
            v-for="destination in releasedDestinations"
            :key="destination.name"
            :to="destination.name === 'jobs' ? lastJobListPath : { name: destination.name }"
            :class="[
              'app-navigation__link',
              {
                'app-navigation__link--active':
                  (destination.name === 'jobs' && isJobDestination) ||
                  destination.name === route.name,
              },
            ]"
            :title="destination.label"
            @click="closeMobileNavigation"
          >
            <span
              class="app-navigation__glyph"
              data-testid="navigation-icon"
              :data-icon="destination.icon"
              aria-hidden="true"
            >
              <svg v-if="destination.icon === 'jobs'" viewBox="0 0 24 24" role="presentation">
                <path d="M7 7.5h10M7 12h10M7 16.5h6" />
                <rect x="3.5" y="3.5" width="17" height="17" rx="3" />
              </svg>
              <svg v-else viewBox="0 0 24 24" role="presentation">
                <path d="M4 17.5 9 12l3.5 3 7.5-8" />
                <path d="M17 7h3v3" />
              </svg>
            </span>
            <span class="app-navigation__label">{{ destination.label }}</span>
          </RouterLink>
        </nav>
        <p class="app-sidebar__note">最新岗位视图</p>
      </aside>
      <div class="app-shell__content">
        <RouterView v-slot="{ Component }">
          <KeepAlive include="JobListView">
            <component :is="Component" />
          </KeepAlive>
        </RouterView>
      </div>
      <AppRequestLoader :visible="requestActivity.visible.value" />
    </div>
  </NConfigProvider>
</template>

<style scoped>
.app-shell {
  background:
    radial-gradient(circle at 4% 0%, rgba(164, 226, 244, 0.52), transparent 24rem),
    linear-gradient(135deg, #edf7fb 0%, #e8f2f8 52%, #eaf7f5 100%);
  color: #121c22;
  display: grid;
  grid-template-columns: 224px minmax(0, 1fr);
  min-height: 100vh;
  overflow-x: clip;
  transition: grid-template-columns 260ms cubic-bezier(0.22, 0.9, 0.25, 1);
}

.app-shell--navigation-collapsed {
  grid-template-columns: 56px minmax(0, 1fr);
}

.app-shell[data-color-scheme='dark'] {
  background:
    radial-gradient(circle at 4% 0%, rgba(38, 117, 143, 0.2), transparent 24rem),
    linear-gradient(135deg, #0d1b22 0%, #10242d 55%, #102927 100%);
  color: #ecf6f7;
}

.app-sidebar {
  background: #f8fcfd;
  border-right: 1px solid #e3ebf0;
  display: flex;
  flex-direction: column;
  height: 100dvh;
  min-width: 0;
  overflow: hidden;
  padding: 22px 10px 20px;
  position: sticky;
  top: 0;
  transition:
    padding 260ms cubic-bezier(0.22, 0.9, 0.25, 1),
    background-color 180ms ease;
  z-index: 2;
}

.app-sidebar__header {
  align-items: flex-start;
  display: flex;
  gap: 6px;
  justify-content: space-between;
  min-width: 0;
}

.app-shell[data-color-scheme='dark'] .app-sidebar {
  background: #102b35;
  border-right-color: #294752;
}

.app-shell__content {
  min-width: 0;
  overflow: visible;
}

.app-sidebar__brand {
  align-items: center;
  display: flex;
  flex: 1 1 auto;
  gap: 10px;
  min-height: 42px;
  min-width: 0;
  padding: 0 8px 26px;
}

.app-sidebar__brand-copy {
  flex: 1 1 auto;
  min-width: 0;
  opacity: 1;
  overflow: hidden;
  transition:
    opacity 180ms ease,
    width 260ms cubic-bezier(0.22, 0.9, 0.25, 1);
  width: auto;
}

.app-sidebar__brand strong,
.app-sidebar__brand small {
  display: block;
}

.app-sidebar__brand strong {
  color: #121c22;
  font-size: 1rem;
  letter-spacing: -0.03em;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__brand strong {
  color: #ecf6f7;
}

.app-sidebar__brand small {
  color: #72818a;
  font-size: 0.58rem;
  font-weight: 700;
  letter-spacing: 0.13em;
  margin-top: 2px;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__brand small,
.app-shell[data-color-scheme='dark'] .app-sidebar__note {
  color: #9bb8c0;
}

.app-sidebar__wave {
  align-items: center;
  display: flex;
  gap: 3px;
  height: 22px;
}

.app-sidebar__wave i {
  background: #007bab;
  border-radius: 99px;
  display: block;
  height: 3px;
  width: 20px;
}

.app-sidebar__wave i:nth-child(2) {
  background: #008287;
  width: 12px;
}

.app-sidebar__wave i:nth-child(3) {
  opacity: 0.42;
  width: 6px;
}

.app-navigation__link {
  align-items: center;
  border-radius: 10px;
  color: #52636d;
  display: flex;
  font-size: 0.9rem;
  font-weight: 650;
  gap: 11px;
  min-height: 46px;
  padding: 0 12px;
  text-decoration: none;
  transition:
    background-color 160ms ease,
    color 160ms ease,
    padding 260ms cubic-bezier(0.22, 0.9, 0.25, 1);
}

.app-shell[data-color-scheme='dark'] .app-navigation__link {
  color: #b6d0d6;
}

.app-navigation__link:hover {
  background: #eaf1f6;
  color: #00658e;
}

.app-shell[data-color-scheme='dark'] .app-navigation__link:hover {
  background: #1e3b47;
  color: #73c9dd;
}

.app-navigation__link--active {
  background: #e4f4fa;
  color: #00658e;
}

.app-shell[data-color-scheme='dark'] .app-navigation__link--active {
  background: #193f4a;
  color: #6ad0c0;
}

.app-navigation__glyph {
  align-items: center;
  color: #008287;
  display: inline-flex;
  flex: 0 0 24px;
  justify-content: center;
  line-height: 1;
}

.app-navigation__glyph svg {
  fill: none;
  height: 20px;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.8;
  width: 20px;
}

.app-navigation__label {
  opacity: 1;
  overflow: hidden;
  transition:
    opacity 180ms ease,
    width 260ms cubic-bezier(0.22, 0.9, 0.25, 1);
  white-space: nowrap;
  width: 130px;
}

.app-sidebar__note {
  border-top: 1px solid #e3ebf0;
  color: #72818a;
  font-size: 0.72rem;
  margin: auto 10px 0;
  padding-top: 16px;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__note {
  border-top-color: #294752;
}

.app-menu-toggle,
.app-navigation-overlay,
.app-sidebar__mobile-close {
  display: none;
}

.app-sidebar__desktop-toggle {
  align-items: center;
  background: transparent;
  border: 0;
  color: #406980;
  cursor: pointer;
  display: flex;
  flex: 0 0 32px;
  font-size: 1.5rem;
  justify-content: center;
  line-height: 1;
  min-height: 32px;
  padding: 0;
  transition:
    background-color 160ms ease,
    color 160ms ease;
}

.app-sidebar__desktop-toggle:hover,
.app-sidebar__desktop-toggle:focus-visible {
  background: #e4f4fa;
  border-radius: 9px;
  color: #00658e;
  outline: none;
}

.app-shell[data-color-scheme='dark'] .app-menu-toggle {
  background: rgba(23, 48, 58, 0.94);
  border-color: #31505c;
}

.app-shell[data-color-scheme='dark'] .app-menu-toggle span {
  background: #6ad0c0;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__desktop-toggle {
  color: #6ad0c0;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__desktop-toggle:hover,
.app-shell[data-color-scheme='dark'] .app-sidebar__desktop-toggle:focus-visible {
  background: #193f4a;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__mobile-close {
  color: #b6d0d6;
}

@media (min-width: 761px) {
  .app-sidebar--collapsed {
    padding-left: 6px;
    padding-right: 6px;
  }

  .app-sidebar--collapsed .app-sidebar__header {
    align-items: center;
  }

  .app-sidebar--collapsed .app-sidebar__brand {
    justify-content: center;
    padding-left: 0;
    padding-right: 0;
  }

  .app-sidebar--collapsed .app-sidebar__brand-copy,
  .app-sidebar--collapsed .app-navigation__label,
  .app-sidebar--collapsed .app-sidebar__note {
    opacity: 0;
    pointer-events: none;
    width: 0;
  }

  .app-sidebar--collapsed .app-sidebar__brand {
    gap: 0;
  }

  .app-sidebar__brand--expand {
    border-radius: 10px;
    cursor: pointer;
    outline: none;
  }

  .app-sidebar__brand--expand:hover,
  .app-sidebar__brand--expand:focus-visible {
    background: #eaf1f6;
  }

  .app-sidebar--collapsed .app-sidebar__wave {
    transform: scale(0.8);
  }

  .app-sidebar--collapsed .app-navigation__link {
    justify-content: center;
    padding-left: 0;
    padding-right: 0;
  }

  .app-sidebar--collapsed .app-navigation__link::after {
    background: transparent;
    border-radius: 0 4px 4px 0;
    content: '';
    inset: 8px auto 8px 0;
    position: absolute;
    width: 3px;
  }

  .app-sidebar--collapsed .app-navigation__link {
    position: relative;
  }

  .app-sidebar--collapsed .app-navigation__link--active::after {
    background: #008287;
  }
}

@media (max-width: 760px) {
  .app-shell {
    display: block;
    height: auto;
    min-height: 100vh;
    overflow: visible;
    transition: none;
  }

  .app-menu-toggle {
    align-items: center;
    background: rgba(255, 255, 255, 0.9);
    border: 1px solid #e3ebf0;
    border-radius: 10px;
    cursor: pointer;
    display: flex;
    flex-direction: column;
    gap: 4px;
    height: 40px;
    justify-content: center;
    left: 16px;
    padding: 0;
    position: fixed;
    top: 14px;
    width: 40px;
    z-index: 30;
  }

  .app-menu-toggle span {
    background: #007bab;
    border-radius: 4px;
    display: block;
    height: 2px;
    width: 17px;
  }

  .app-navigation-overlay {
    background: rgba(11, 37, 50, 0.3);
    display: block;
    inset: 0;
    position: fixed;
    z-index: 40;
  }

  .app-sidebar {
    box-shadow: 18px 0 42px rgba(11, 55, 76, 0.18);
    height: 100vh;
    left: 0;
    min-height: 0;
    position: fixed;
    top: 0;
    transition: transform 180ms ease;
    width: min(280px, 82vw);
    z-index: 50;
  }

  .app-sidebar--mobile-open {
    transform: translateX(0);
  }

  .app-sidebar:not(.app-sidebar--mobile-open) {
    transform: translateX(-105%);
  }

  .app-sidebar__brand-copy,
  .app-navigation__label {
    opacity: 1;
    width: auto;
  }

  .app-shell__content {
    min-height: 0;
    overflow: visible;
    padding-top: 24px;
  }

  .app-sidebar__desktop-toggle {
    display: none;
  }

  .app-sidebar__mobile-close {
    background: transparent;
    border: 0;
    color: #406980;
    cursor: pointer;
    display: block;
    font-size: 1.6rem;
    line-height: 1;
    margin-left: auto;
    padding: 4px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-shell,
  .app-sidebar,
  .app-navigation__link {
    transition-duration: 140ms !important;
  }

  .app-sidebar__brand-copy,
  .app-navigation__label {
    transition: opacity 140ms ease !important;
  }
}
</style>
