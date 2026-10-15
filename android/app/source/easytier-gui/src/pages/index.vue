<script setup lang="ts">
import { NetworkTypes, Utils } from 'easytier-frontend-lib'
import { type as osType } from '@tauri-apps/plugin-os'

import { runNetworkInstance, retainNetworkInstance, collectNetworkInfos } from '~/composables/network'
import { prepareVpnService, initMobileVpnService } from '~/composables/mobile_vpn'

const networkStore = useNetworkStore()

const errorMsg = ref('')
const busy = ref(false)
const bootstrapDraft = ref('')

const LS_KEY = 'pnet.networks.v1'

function seedDefault(): NetworkTypes.NetworkConfig {
  return NetworkTypes.DEFAULT_NETWORK_CONFIG()
}

function loadNetworks() {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (raw) {
      const list = JSON.parse(raw)
      if (Array.isArray(list) && list.length) {
        networkStore.networkList = list.map((c: any) => ({ ...seedDefault(), ...c }))
        networkStore.curNetwork = networkStore.networkList[0]
        return
      }
    }
  } catch {
    /* ignore */
  }
  networkStore.networkList = [seedDefault()]
  networkStore.curNetwork = networkStore.networkList[0]
}

function persist() {
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(networkStore.networkList))
  } catch {
    /* ignore */
  }
}

networkStore.$subscribe(persist)

const cfg = computed(() => networkStore.curNetwork)
const instanceId = computed(() => networkStore.curNetwork.instance_id)
const running = computed(() => networkStore.networkInstanceIds.includes(instanceId.value))
const isManual = computed(() => cfg.value.networking_method === NetworkTypes.NetworkingMethod.Manual)

const coreError = computed(() => {
  const info = networkStore.networkInfos[instanceId.value]
  return info?.error_msg || ''
})

function fmtIp(ip: any): string {
  if (!ip) return ''
  if (typeof ip === 'string') return ip
  try {
    return Utils.ipv4InetToString(ip)
  } catch {
    return ''
  }
}

const myIp = computed(() => {
  const info = networkStore.networkInfos[instanceId.value]
  const v = info?.my_node_info?.virtual_ipv4
  return v ? Utils.ipv4InetToString(v) : ''
})

const peers = computed(() => {
  const info = networkStore.networkInfos[instanceId.value]
  const prs = (info?.peer_route_pairs as any[]) || []
  return prs
    .filter((p) => p?.route)
    .map((p) => ({
      hostname: p.route.hostname || (p.peer ? `peer ${p.peer.peer_id}` : ''),
      ip: fmtIp(p.route.ipv4_addr),
    }))
})

const peerCount = computed(() => peers.value.length)

async function updateInfos() {
  try {
    networkStore.updateWithNetworkInfos(await collectNetworkInfos())
  } catch {
    /* ignore */
  }
}

async function connect() {
  errorMsg.value = ''
  busy.value = true
  const id = instanceId.value
  try {
    if (osType() === 'android') {
      await prepareVpnService(id)
      networkStore.clearNetworkInstances()
    } else {
      networkStore.removeNetworkInstance(id)
    }

    await retainNetworkInstance(networkStore.networkInstanceIds)
    networkStore.addNetworkInstance(id)

    try {
      await runNetworkInstance(networkStore.curNetwork)
      networkStore.addAutoStartInstId(id)
    } catch (e: any) {
      errorMsg.value = String(e?.message ?? e)
    }
  } finally {
    busy.value = false
  }
}

async function disconnect() {
  busy.value = true
  try {
    networkStore.removeNetworkInstance(instanceId.value)
    await retainNetworkInstance(networkStore.networkInstanceIds)
    networkStore.removeAutoStartInstId(instanceId.value)
  } finally {
    busy.value = false
  }
}

function addNetwork() {
  networkStore.networkList.push(seedDefault())
  networkStore.curNetwork = networkStore.networkList[networkStore.networkList.length - 1]
}

function delNetwork() {
  if (networkStore.networkList.length <= 1) return
  const idx = networkStore.networkList.indexOf(networkStore.curNetwork)
  networkStore.networkList.splice(idx, 1)
  networkStore.curNetwork = networkStore.networkList[Math.max(0, idx - 1)]
}

function addBootstrap() {
  const u = bootstrapDraft.value.trim()
  if (!u || cfg.value.peer_urls.includes(u)) return
  cfg.value.peer_urls.push(u)
  bootstrapDraft.value = ''
}

function removeBootstrap(u: string) {
  const i = cfg.value.peer_urls.indexOf(u)
  if (i >= 0) cfg.value.peer_urls.splice(i, 1)
}

let intervalId = 0
onMounted(async () => {
  loadNetworks()
  intervalId = window.setInterval(updateInfos, 1000)
  if (osType() === 'android') {
    try {
      await initMobileVpnService()
    } catch (e) {
      console.error('init vpn service failed', e)
    }
  }
  await updateInfos()
})
onUnmounted(() => clearInterval(intervalId))
</script>

<template>
  <div class="pnet-root">
    <header class="pnet-header">
      <div class="brand">
        <span class="brand-dot" :class="{ on: running }"></span>
        <span class="brand-name">pnet</span>
      </div>
      <Tag :severity="running ? 'success' : 'info'" :value="running ? 'в сети' : 'оффлайн'" />
    </header>

    <main class="pnet-main">
      <!-- network picker -->
      <section class="pnet-card picker">
        <Select v-model="networkStore.curNetwork" :options="networkStore.networkList" class="net-select"
          :disabled="running">
          <template #value="slotProps">
            <span v-if="slotProps.value"> {{ slotProps.value.network_name || '(без имени)' }}</span>
          </template>
          <template #option="slotProps">
            <span>{{ slotProps.option.network_name || '(без имени)' }}</span>
          </template>
        </Select>
        <Button icon="pi pi-plus" rounded text :disabled="running" @click="addNetwork" />
        <Button icon="pi pi-trash" rounded text severity="danger" :disabled="running || networkStore.networkList.length <= 1"
          @click="delNetwork" />
      </section>

      <!-- editor -->
      <section class="pnet-card">
        <div class="field">
          <label>Имя сети</label>
          <InputText v-model="cfg.network_name" :disabled="running" placeholder="имя сети" />
        </div>

        <div class="field">
          <label>Пароль сети</label>
          <Password v-model="cfg.network_secret" :disabled="running" :feedback="false" toggle-mask
            placeholder="оставьте пустым, если нет" />
        </div>

        <div class="field">
          <label>DNS</label>
          <InputText v-model="cfg.dns" :disabled="running" placeholder="напр. 10.126.0.1" class="mono" />
        </div>

        <div class="field">
          <label>Имя узла (никнейм)</label>
          <InputText v-model="cfg.hostname" :disabled="running" placeholder="пусто = имя устройства" class="mono" />
        </div>

        <div class="field">
          <label>Способ подключения</label>
          <SelectButton v-model="cfg.networking_method"
            :options="[NetworkTypes.NetworkingMethod.Manual, NetworkTypes.NetworkingMethod.PublicServer]"
            :option-label="(v) => v === NetworkTypes.NetworkingMethod.Manual ? 'Свои ноды' : 'Публичный сервер'"
            :disabled="running" />
        </div>

        <div v-if="isManual" class="field">
          <label>Bootstrap-ноды</label>
          <div class="chips">
            <Tag v-for="u in cfg.peer_urls" :key="u" :value="u" class="chip">
              <template #default>
                <span class="chip-text">{{ u }}</span>
                <i v-if="!running" class="pi pi-times chip-x" @click="removeBootstrap(u)" />
              </template>
            </Tag>
          </div>
          <div class="add-row" v-if="!running">
            <InputText v-model="bootstrapDraft" placeholder="udp://1.2.3.4:11010" class="mono grow"
              @keyup.enter="addBootstrap" />
            <Button icon="pi pi-plus" label="Добавить" @click="addBootstrap" />
          </div>
        </div>

        <div v-else class="field">
          <label>Публичный сервер</label>
          <InputText v-model="cfg.public_server_url" :disabled="running" placeholder="tcp://public.easytier.top:11010"
            class="mono" />
        </div>

        <Button class="connect-btn" :label="running ? 'Отключиться' : 'Подключиться'"
          :severity="running ? 'danger' : 'success'" :icon="running ? 'pi pi-power-off' : 'pi pi-play'" :loading="busy"
          @click="running ? disconnect() : connect()" />
      </section>

      <!-- status -->
      <section v-if="running" class="pnet-card status">
        <div class="status-row">
          <span>Ваш IP</span>
          <b class="mono">{{ myIp || '—' }}</b>
        </div>
        <div class="status-row">
          <span>Соседей в сети</span>
          <b>{{ peerCount }}</b>
        </div>

        <DataTable v-if="peers.length" :value="peers" class="peers" size="small">
          <Column field="hostname" header="Имя" />
          <Column field="ip" header="IP" />
        </DataTable>
        <p v-else class="hint">Соседей нет. Проверьте, что bootstrap-ноды доступны из сети телефона
          (UDP&nbsp;11010) и что имя/пароль сети совпадают.</p>
      </section>

      <InlineMessage v-if="errorMsg" severity="error" class="err-msg">{{ errorMsg }}</InlineMessage>
      <InlineMessage v-else-if="coreError" severity="warn" class="err-msg">{{ coreError }}</InlineMessage>
    </main>
  </div>
</template>

<style scoped lang="postcss">
.pnet-root {
  height: 100vh;
  width: 100vw;
  display: flex;
  flex-direction: column;
  background: radial-gradient(120% 80% at 50% 0%, #0f2a1d 0%, #07140e 60%, #050d09 100%);
  color: #e8f5ee;
  overflow: hidden;
}

.pnet-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid rgba(74, 222, 128, 0.15);
}

.brand {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.brand-name {
  font-size: 1.4rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #4ade80;
}

.brand-dot {
  width: 0.8rem;
  height: 0.8rem;
  border-radius: 9999px;
  background: #3b5147;
  box-shadow: 0 0 0 3px rgba(74, 222, 128, 0.12);
  transition: background 0.25s;
}

.brand-dot.on {
  background: #4ade80;
  box-shadow: 0 0 10px 2px rgba(74, 222, 128, 0.55);
}

.pnet-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.25rem;
  overflow-y: auto;
}

.pnet-card {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(74, 222, 128, 0.12);
  border-radius: 16px;
  padding: 1.1rem 1.1rem 1.3rem;
  display: flex;
  flex-direction: column;
  gap: 0.95rem;
}

.picker {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
  padding: 0.7rem 0.8rem;
}

.net-select {
  flex: 1;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.field label {
  font-size: 0.8rem;
  color: #9fcdb4;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.chip {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.72rem;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}

.chip-x {
  cursor: pointer;
  opacity: 0.7;
}

.chip-x:hover {
  opacity: 1;
}

.add-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.grow {
  flex: 1;
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.connect-btn {
  margin-top: 0.35rem;
  width: 100%;
}

.status-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.95rem;
}

.status-row span {
  color: #9fcdb4;
}

.peers {
  margin-top: 0.4rem;
  background: transparent;
}

.hint {
  font-size: 0.8rem;
  color: #9fcdb4;
  line-height: 1.4;
}

.err-msg {
  align-self: flex-start;
}
</style>
