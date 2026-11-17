#!/usr/bin/env python3
"""Verify the trend dashboard's responsive geometry through a local Chrome CDP page."""

from __future__ import annotations

import base64
import json
import itertools
import sys
import time
import urllib.request
from pathlib import Path

try:
    import websocket
except ImportError as error:  # pragma: no cover - exercised only outside the dev image
    raise SystemExit(
        'websocket-client is required; install it in the local verification environment'
    ) from error


CDP_PORT = 9227
VIEWPORTS = (1440, 1280, 1100, 900, 390)


class ChromePage:
    def __init__(self, websocket_url: str) -> None:
        self.socket = websocket.create_connection(websocket_url)
        self.sequence = itertools.count(1)

    def call(self, method: str, params: dict | None = None) -> dict:
        request_id = next(self.sequence)
        self.socket.send(
            json.dumps({'id': request_id, 'method': method, 'params': params or {}})
        )
        while True:
            message = json.loads(self.socket.recv())
            if message.get('id') == request_id:
                return message

    def evaluate(self, expression: str):
        result = self.call(
            'Runtime.evaluate',
            {'awaitPromise': True, 'returnByValue': True, 'expression': expression},
        )
        remote = result['result']['result']
        if 'value' not in remote:
            raise RuntimeError(remote.get('description', 'CDP evaluation failed'))
        return remote['value']

    def set_viewport(self, width: int) -> None:
        self.call(
            'Emulation.setDeviceMetricsOverride',
            {'width': width, 'height': 900, 'deviceScaleFactor': 1, 'mobile': False},
        )

    def clear_viewport(self) -> None:
        self.call('Emulation.clearDeviceMetricsOverride')

    def close(self) -> None:
        self.socket.close()


def find_trend_page() -> ChromePage:
    pages = json.load(urllib.request.urlopen(f'http://127.0.0.1:{CDP_PORT}/json/list'))
    page = next((item for item in pages if item.get('type') == 'page' and '/job-trends' in item.get('url', '')), None)
    if page is None:
        raise SystemExit('No local /job-trends page found on Chrome CDP 9227')
    return ChromePage(page['webSocketDebuggerUrl'])


GEOMETRY = """
(() => {
  const query = (selector) => document.querySelector(selector)
  const rect = (selector) => query(selector)?.getBoundingClientRect().toJSON()
  const rail = query('[data-testid="trend-configuration-rail"]')
  const trigger = query('[data-testid="open-trend-configuration"]')
  return {
    width: innerWidth,
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
    chart: rect('[data-testid="trend-chart-region"]'),
    rail: rail && {
      display: getComputedStyle(rail).display,
      rect: rail.getBoundingClientRect().toJSON(),
      open: rail.classList.contains('job-trend-view__rail--drawer-open'),
    },
    trigger: trigger && {
      display: getComputedStyle(trigger).display,
      rect: trigger.getBoundingClientRect().toJSON(),
    },
  }
})()
"""


def assert_geometry(page: ChromePage, width: int, geometry: dict) -> None:
    if geometry['scrollWidth'] != geometry['clientWidth']:
        raise AssertionError(f'{width}px has horizontal overflow: {geometry}')
    chart_width = geometry['chart']['width']
    if width >= 1440 and chart_width < 640:
        raise AssertionError(f'{width}px wide chart is too narrow: {chart_width}')
    if 1180 <= width <= 1439 and chart_width < 520:
        raise AssertionError(f'{width}px standard chart is too narrow: {chart_width}')
    if geometry['rail']['display'] != 'none':
        raise AssertionError(f'{width}px configuration rail is visible while closed: {geometry}')
    if geometry['trigger']['display'] == 'none' or geometry['trigger']['rect']['width'] <= 0:
        raise AssertionError(f'{width}px configuration trigger is unavailable: {geometry}')


def verify_navigation(page: ChromePage) -> None:
    page.evaluate('document.querySelector(\'[data-testid="collapse-desktop-navigation"]\')?.click(); true')
    collapsed = page.evaluate(
        """
        (() => ({
          sidebar: document.querySelector('.app-sidebar')?.className,
          trigger: document.querySelector('[data-testid="expand-desktop-navigation"]')?.getBoundingClientRect().toJSON(),
        }))()
        """
    )
    if 'app-sidebar--collapsed' not in collapsed['sidebar']:
        raise AssertionError(f'desktop navigation is not collapsed: {collapsed}')
    if collapsed['trigger'] is None or collapsed['trigger']['width'] <= 0:
        raise AssertionError(f'desktop navigation trigger is unavailable: {collapsed}')
    page.evaluate('document.querySelector(\'[data-testid="expand-desktop-navigation"]\')?.click(); true')
    expanded = page.evaluate(
        """
        (() => ({
          sidebar: document.querySelector('.app-sidebar')?.className,
          collapse: Boolean(document.querySelector('[data-testid="collapse-desktop-navigation"]')),
        }))()
        """
    )
    if 'app-sidebar--collapsed' in expanded['sidebar'] or not expanded['collapse']:
        raise AssertionError(f'desktop navigation did not open: {expanded}')
    page.evaluate('document.querySelector(\'[data-testid="collapse-desktop-navigation"]\')?.click(); true')


def verify_drawer(page: ChromePage) -> None:
    page.set_viewport(900)
    page.evaluate('document.querySelector(\'[data-testid="start-first-curve"]\')?.click(); true')
    time.sleep(0.2)
    page.evaluate('document.querySelector(\'[data-testid="close-trend-configuration"]\')?.click(); true')
    page.evaluate('document.querySelector(\'[data-testid="open-trend-configuration"]\')?.click(); true')
    time.sleep(0.25)
    opened = page.evaluate(
        """
        (() => {
          const rail = document.querySelector('[data-testid="trend-configuration-rail"]')
          const list = document.querySelector('.job-trend-view__curve-list')
          const footer = document.querySelector('[data-testid="curve-rail-footer"]')
          return {
            rail: rail.getBoundingClientRect().toJSON(),
            list: list ? {height: list.clientHeight, scrollHeight: list.scrollHeight, overflow: getComputedStyle(list).overflowY} : null,
            footer: footer ? footer.getBoundingClientRect().toJSON() : null,
            backdrop: Boolean(document.querySelector('[data-testid="trend-configuration-backdrop"]')),
          }
        })()
        """
    )
    if opened['rail']['right'] > 900 or opened['rail']['left'] < 0 or not opened['backdrop']:
        raise AssertionError(f'900px drawer is outside the viewport: {opened}')
    if opened['list'] is not None and opened['list']['overflow'] not in ('auto', 'scroll'):
        raise AssertionError(f'curve list is not independently scrollable: {opened}')
    if opened['footer'] is not None and opened['footer']['bottom'] > 900:
        raise AssertionError(f'drawer footer is unreachable: {opened}')

    page.set_viewport(1280)
    time.sleep(0.2)
    transitioned = page.evaluate(
        """
        (() => {
          const rail = document.querySelector('[data-testid="trend-configuration-rail"]')
          return {
            clientWidth: document.documentElement.clientWidth,
            display: getComputedStyle(rail).display,
            right: rail.getBoundingClientRect().right,
          }
        })()
        """
    )
    if transitioned['display'] != 'flex' or transitioned['right'] > transitioned['clientWidth']:
        raise AssertionError(f'open configuration did not settle at standard desktop: {transitioned}')


def verify_mobile_overlays(page: ChromePage) -> bool:
    page.set_viewport(390)
    page.evaluate('document.querySelector(\'[aria-label="关闭岗位详情"]\')?.click(); true')
    page.evaluate('document.querySelector(\'[aria-label="关闭节点岗位"]\')?.click(); true')
    time.sleep(0.2)
    event_button = page.evaluate(
        "Boolean(document.querySelector('.job-trend-view__event-table td button'))"
    )
    if not event_button:
        return False
    page.evaluate('document.querySelector(\'.job-trend-view__event-table td button\')?.click(); true')
    time.sleep(0.35)
    drilldown = page.evaluate(
        """
        (() => {
          const element = document.querySelector('.job-trend-view__drilldown')
          return element?.getBoundingClientRect().toJSON()
        })()
        """
    )
    if drilldown is None or drilldown['left'] < 0 or drilldown['right'] > 390 or drilldown['bottom'] > 900:
        raise AssertionError(f'390px drilldown is outside the viewport: {drilldown}')
    page.evaluate('document.querySelector(\'.job-trend-view__drilldown-list button\')?.click(); true')
    time.sleep(0.35)
    detail = page.evaluate(
        """
        (() => {
          const element = document.querySelector('.job-trend-view__detail-drawer')
          return element?.getBoundingClientRect().toJSON()
        })()
        """
    )
    if detail is None or detail['left'] < 0 or detail['right'] > 390 or detail['top'] < 0 or detail['bottom'] > 900:
        raise AssertionError(f'390px detail drawer is outside the viewport: {detail}')
    return True


def main() -> int:
    page = find_trend_page()
    try:
        page.evaluate('document.querySelector(\'[data-testid="close-trend-configuration"]\')?.click(); true')
        time.sleep(0.2)
        verify_navigation(page)
        for width in VIEWPORTS:
            page.set_viewport(width)
            time.sleep(0.15)
            geometry = page.evaluate(GEOMETRY)
            assert_geometry(page, width, geometry)
            print(f'PASS {width}px chart={geometry["chart"]["width"]:.0f}px overflow=0')
        verify_drawer(page)
        mobile_overlays_verified = verify_mobile_overlays(page)
        if mobile_overlays_verified:
            print('PASS compact drawer, mobile drilldown, mobile detail, and breakpoint transition')
        else:
            print('PASS side-panel interactions and responsive geometry; mobile drilldown skipped without trend fixture data')
    finally:
        page.clear_viewport()
        page.close()
    return 0


if __name__ == '__main__':
    sys.exit(main())
