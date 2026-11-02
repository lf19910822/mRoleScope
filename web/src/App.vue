<script setup lang="ts">
import { NConfigProvider } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { oceanBreezeThemeOverrides } from '@/theme/ocean-breeze'

const route = useRoute()
const navigationOpen = ref(false)
const releasedDestinations = [{ label: '岗位筛选', name: 'jobs' as const }]
const isJobDestination = computed(() => route.name === 'jobs' || route.name === 'job-detail')

function closeNavigation(): void {
  navigationOpen.value = false
}

watch(
  () => route.fullPath,
  () => closeNavigation(),
)
</script>

<template>
  <NConfigProvider :theme-overrides="oceanBreezeThemeOverrides">
    <div class="app-shell">
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
    </div>
  </NConfigProvider>
</template>

<style scoped>
.app-shell {
  display: grid;
  grid-template-columns: 236px minmax(0, 1fr);
  min-height: 100vh;
}

.app-sidebar {
  background: rgba(255, 255, 255, 0.92);
  border-right: 1px solid #e3ebf0;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  padding: 28px 16px 20px;
  position: sticky;
  top: 0;
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

.app-sidebar__brand small {
  color: #72818a;
  font-size: 0.58rem;
  font-weight: 700;
  letter-spacing: 0.13em;
  margin-top: 2px;
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
  transition: background-color 160ms ease, color 160ms ease;
}

.app-navigation__link:hover {
  background: #eaf1f6;
  color: #00658e;
}

.app-navigation__link--active {
  background: #e4f4fa;
  color: #00658e;
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

.app-menu-toggle,
.app-sidebar__close,
.app-navigation-overlay {
  display: none;
}

@media (max-width: 760px) {
  .app-shell {
    display: block;
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
