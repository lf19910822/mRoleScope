<script setup lang="ts">
import { darkTheme, dateZhCN, NConfigProvider, useOsTheme, zhCN } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { oceanBreezeDarkThemeOverrides, oceanBreezeThemeOverrides } from '@/theme/ocean-breeze'
import AppRequestLoader from '@/components/AppRequestLoader.vue'
import { requestActivity } from '@/lib/request-activity'

const route = useRoute()
const osTheme = useOsTheme()
const navigationOpen = ref(false)
const releasedDestinations = [{ label: '岗位筛选', name: 'jobs' as const }]
const isJobDestination = computed(() => route.name === 'jobs' || route.name === 'job-detail')
const colorScheme = computed(() => (osTheme.value === 'dark' ? 'dark' : 'light'))
const selectedTheme = computed(() => (colorScheme.value === 'dark' ? darkTheme : null))
const selectedThemeOverrides = computed(() =>
  colorScheme.value === 'dark' ? oceanBreezeDarkThemeOverrides : oceanBreezeThemeOverrides,
)

function closeNavigation(): void {
  navigationOpen.value = false
}

watch(
  () => route.fullPath,
  () => closeNavigation(),
)
</script>

<template>
  <NConfigProvider
    :date-locale="dateZhCN"
    :locale="zhCN"
    :theme="selectedTheme"
    :theme-overrides="selectedThemeOverrides"
  >
    <div class="app-shell" :data-color-scheme="colorScheme">
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
        @click="closeNavigation"
      ></div>
      <aside :class="['app-sidebar', { 'app-sidebar--open': navigationOpen }]">
        <div class="app-sidebar__brand">
          <span class="app-sidebar__wave" aria-hidden="true"><i></i><i></i><i></i></span>
          <div>
            <strong>mRoleScope</strong>
            <small>JD OBSERVATORY</small>
          </div>
          <button
            class="app-sidebar__close"
            type="button"
            aria-label="关闭导航"
            data-testid="close-navigation"
            @click="closeNavigation"
          >
            ×
          </button>
        </div>
        <nav id="application-navigation" aria-label="主导航" data-testid="application-navigation">
          <RouterLink
            v-for="destination in releasedDestinations"
            :key="destination.name"
            :to="{ name: destination.name }"
            :class="[
              'app-navigation__link',
              { 'app-navigation__link--active': destination.name === 'jobs' && isJobDestination },
            ]"
            @click="closeNavigation"
          >
            <span class="app-navigation__glyph" aria-hidden="true">⌁</span>
            {{ destination.label }}
          </RouterLink>
        </nav>
        <p class="app-sidebar__note">最新岗位视图</p>
      </aside>
      <div class="app-shell__content">
        <RouterView />
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
  grid-template-columns: 236px minmax(0, 1fr);
  min-height: 100vh;
}

.app-shell[data-color-scheme='dark'] {
  background:
    radial-gradient(circle at 4% 0%, rgba(38, 117, 143, 0.2), transparent 24rem),
    linear-gradient(135deg, #0d1b22 0%, #10242d 55%, #102927 100%);
  color: #ecf6f7;
}

.app-sidebar {
  background: rgba(255, 255, 255, 0.92);
  border-right: 1px solid #e3ebf0;
  display: flex;
  flex-direction: column;
  height: 100dvh;
  left: 0;
  padding: 28px 16px 20px;
  position: fixed;
  top: 0;
  width: 236px;
  z-index: 10;
}

.app-shell[data-color-scheme='dark'] .app-sidebar {
  background: rgba(16, 37, 46, 0.94);
  border-right-color: #294752;
}

.app-shell__content {
  grid-column: 2;
  min-width: 0;
  overflow: visible;
}

.app-sidebar__brand {
  align-items: center;
  display: flex;
  gap: 10px;
  padding: 0 10px 30px;
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
    color 160ms ease;
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
  color: #008287;
  font-size: 1.35rem;
  line-height: 1;
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
.app-sidebar__close,
.app-navigation-overlay {
  display: none;
}

.app-shell[data-color-scheme='dark'] .app-menu-toggle {
  background: rgba(23, 48, 58, 0.94);
  border-color: #31505c;
}

.app-shell[data-color-scheme='dark'] .app-menu-toggle span {
  background: #6ad0c0;
}

.app-shell[data-color-scheme='dark'] .app-sidebar__close {
  color: #b6d0d6;
}

@media (max-width: 760px) {
  .app-shell {
    display: block;
    height: auto;
    min-height: 100vh;
    overflow: visible;
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
    transform: translateX(-105%);
    transition: transform 180ms ease;
    width: min(280px, 82vw);
    z-index: 50;
  }

  .app-sidebar--open {
    transform: translateX(0);
  }

  .app-shell__content {
    min-height: 0;
    overflow: visible;
    padding-top: 24px;
  }

  .app-sidebar__close {
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
</style>
