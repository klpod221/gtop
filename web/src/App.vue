<template>
  <div class="min-h-screen relative font-sans selection:bg-indigo-500/30">
    <!-- Background elements for modern glass effect -->
    <div class="fixed inset-0 z-[-1] overflow-hidden pointer-events-none">
      <div class="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] rounded-full bg-indigo-500/10 dark:bg-indigo-500/20 blur-[100px] mix-blend-multiply dark:mix-blend-screen transition-all duration-1000"></div>
      <div class="absolute top-[20%] right-[-10%] w-[50%] h-[50%] rounded-full bg-purple-500/10 dark:bg-purple-600/20 blur-[120px] mix-blend-multiply dark:mix-blend-screen transition-all duration-1000"></div>
      <div class="absolute bottom-[-10%] left-[20%] w-[60%] h-[60%] rounded-full bg-blue-500/10 dark:bg-blue-600/20 blur-[120px] mix-blend-multiply dark:mix-blend-screen transition-all duration-1000"></div>
    </div>

    <!-- Header -->
    <header class="sticky top-0 z-50 glass-panel border-b border-gray-200/50 dark:border-white/10 rounded-none shadow-sm">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-8 h-8 rounded-lg bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center shadow-lg shadow-indigo-500/30">
            <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path></svg>
          </div>
          <h1 class="text-xl font-bold tracking-tight bg-clip-text text-transparent bg-gradient-to-r from-gray-900 to-gray-600 dark:from-white dark:to-gray-300">gtop</h1>
          <span class="px-2.5 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider transition-colors duration-300 ml-2" :class="store.isConnected ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-500/20 dark:text-emerald-400' : 'bg-rose-100 text-rose-800 dark:bg-rose-500/20 dark:text-rose-400'">
            {{ store.isConnected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>
        
        <div class="flex items-center gap-4">
          <div v-if="payload?.host" class="hidden md:flex flex-col items-end mr-4">
            <span class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ payload.host.hostname }}</span>
            <span class="text-xs text-gray-500 dark:text-gray-400 font-medium opacity-80">{{ payload.host.os }} • {{ payload.host.kernel_version }}</span>
          </div>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
      <div v-if="!payload" class="flex flex-col items-center justify-center h-64 gap-5">
        <div class="w-14 h-14 border-4 border-indigo-500/20 border-t-indigo-500 rounded-full animate-spin shadow-lg shadow-indigo-500/20"></div>
        <p class="text-gray-500 dark:text-gray-400 font-medium animate-pulse">Waiting for telemetry data...</p>
      </div>

      <template v-else>
        <!-- Top Metrics Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <MetricCard title="CPU Usage" :value="payload.cpu?.usage_percent?.toFixed(1) || 0" unit="%" :subtitle="payload.cpu?.cpu_name" />
          <MetricCard title="Memory" :value="formatBytes(payload.memory?.used || 0)" :unit="'/ ' + formatBytes(payload.memory?.total || 0)" :subtitle="((payload.memory?.used / payload.memory?.total) * 100).toFixed(1) + '% Used'" />
          <MetricCard title="Network Rx" :value="formatBytes(netRxRate)" unit="/s" subtitle="Total Download Speed" />
          <MetricCard title="Network Tx" :value="formatBytes(netTxRate)" unit="/s" subtitle="Total Upload Speed" />
        </div>

        <!-- Charts Row -->
        <div class="grid grid-cols-1 xl:grid-cols-2 gap-6 h-[320px]">
          <ChartLine title="CPU History (%)" :data="cpuHistory" color="#6366f1" :isDark="true" />
          <ChartLine title="Memory History (GB)" :data="memHistory" color="#a855f7" :isDark="true" />
        </div>

        <!-- Bottom Row: Storage & Processes -->
        <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
          <div class="xl:col-span-1 space-y-6 flex flex-col">
            <!-- Disks -->
            <div class="glass-panel p-6 rounded-2xl border border-gray-200/50 dark:border-white/10 shadow-lg flex-1">
              <h3 class="text-sm font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider mb-5">Storage Mounts</h3>
              <div class="space-y-5">
                <div v-for="disk in payload.disks_space" :key="disk.mount_point" class="space-y-2">
                   <div class="flex justify-between text-sm items-end">
                    <span class="text-gray-700 dark:text-gray-300 font-semibold">{{ disk.mount_point }}</span>
                    <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ formatBytes(disk.used_bytes) }} / {{ formatBytes(disk.total_bytes) }}</span>
                  </div>
                  <div class="w-full bg-gray-100 dark:bg-gray-800/80 rounded-full h-2 overflow-hidden shadow-inner border border-gray-200/50 dark:border-gray-700/50">
                    <div class="bg-gradient-to-r from-teal-400 to-emerald-500 h-2 rounded-full transition-all duration-500 ease-out" :style="{ width: disk.used_pct + '%' }"></div>
                  </div>
                </div>
              </div>
            </div>

            <!-- GPU Info if available -->
            <div v-if="payload.intel_gpu" class="glass-panel p-6 rounded-2xl border border-gray-200/50 dark:border-white/10 shadow-lg mt-6">
              <h3 class="text-sm font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider mb-5">Intel GPU PMU</h3>
              <div class="space-y-5">
                <div class="flex justify-between items-center px-4 py-3 bg-white/40 dark:bg-gray-800/40 rounded-xl border border-gray-100 dark:border-gray-700/50">
                  <span class="text-xs font-semibold uppercase tracking-wider text-gray-500">Frequency</span>
                  <span class="font-bold text-indigo-600 dark:text-indigo-400">{{ payload.intel_gpu.freq_act_mhz || 0 }} MHz</span>
                </div>
                <div class="flex justify-between items-center px-4 py-3 bg-white/40 dark:bg-gray-800/40 rounded-xl border border-gray-100 dark:border-gray-700/50">
                  <span class="text-xs font-semibold uppercase tracking-wider text-gray-500">Power</span>
                  <span class="font-bold text-rose-600 dark:text-rose-400">{{ payload.intel_gpu.power_gpu_watts?.toFixed(1) || 0 }} W</span>
                </div>
                <div v-for="engine in payload.intel_gpu.engines" :key="engine.name" class="space-y-2 px-1">
                  <div class="flex justify-between text-sm">
                    <span class="text-gray-700 dark:text-gray-300 font-semibold">{{ engine.name }}</span>
                    <span class="font-bold text-gray-800 dark:text-gray-200">{{ engine.busy_pct?.toFixed(1) || 0 }}%</span>
                  </div>
                  <div class="w-full bg-gray-100 dark:bg-gray-800/80 rounded-full h-2 overflow-hidden shadow-inner border border-gray-200/50 dark:border-gray-700/50">
                    <div class="bg-gradient-to-r from-indigo-400 to-purple-500 h-2 rounded-full transition-all duration-500 ease-out" :style="{ width: (engine.busy_pct || 0) + '%' }"></div>
                  </div>
                </div>
              </div>
            </div>
            
            <div v-if="payload.nvidia_gpus && payload.nvidia_gpus.length > 0" class="glass-panel p-6 rounded-2xl border border-gray-200/50 dark:border-white/10 shadow-lg mt-6">
              <h3 class="text-sm font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider mb-5">NVIDIA GPU</h3>
              <div v-for="gpu in payload.nvidia_gpus" :key="gpu.uuid" class="space-y-3">
                 <div class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ gpu.name }}</div>
                 <div class="flex justify-between text-xs">
                    <span class="text-gray-500">Usage</span>
                    <span class="font-bold text-gray-700 dark:text-gray-300">{{ gpu.util_gpu }}%</span>
                 </div>
                 <div class="w-full bg-gray-100 dark:bg-gray-800/80 rounded-full h-2 overflow-hidden shadow-inner border border-gray-200/50 dark:border-gray-700/50">
                    <div class="bg-gradient-to-r from-green-400 to-emerald-500 h-2 rounded-full transition-all duration-500 ease-out" :style="{ width: gpu.util_gpu + '%' }"></div>
                 </div>
              </div>
            </div>
            
          </div>
          
          <div class="xl:col-span-2">
            <ProcessTable :processes="payload.processes" />
          </div>
        </div>
      </template>
    </main>
  </div>
</template>

<script setup>
import { computed, watch, ref, onMounted } from 'vue'
import { useMetricsStore } from './store.js'
import MetricCard from './components/MetricCard.vue'
import ProcessTable from './components/ProcessTable.vue'
import ChartLine from './components/ChartLine.vue'

const store = useMetricsStore()
const payload = computed(() => store.payload)

// History data for charts
const MAX_HISTORY = 60
const cpuHistory = ref([])
const memHistory = ref([])

// Network specific calculations
let lastNet = null
const netRxRate = ref(0)
const netTxRate = ref(0)

watch(() => store.payload, (newPayload) => {
  if (!newPayload) return
  const now = Date.now()
  
  // Update CPU history
  if (newPayload.cpu) {
    cpuHistory.value.push({ time: now, value: newPayload.cpu.usage_percent })
    if (cpuHistory.value.length > MAX_HISTORY) cpuHistory.value.shift()
  }
  
  // Update Mem history
  if (newPayload.memory) {
    memHistory.value.push({ time: now, value: newPayload.memory.used / (1024 * 1024 * 1024) })
    if (memHistory.value.length > MAX_HISTORY) memHistory.value.shift()
  }
  
  // Calculate network rate
  if (newPayload.network) {
    let currentRx = 0
    let currentTx = 0
    newPayload.network.forEach(net => {
      currentRx += net.bytes_recv
      currentTx += net.bytes_sent
    })
    
    if (lastNet) {
      const dt = (now - lastNet.time) / 1000
      if (dt > 0) {
        netRxRate.value = Math.max(0, (currentRx - lastNet.rx) / dt)
        netTxRate.value = Math.max(0, (currentTx - lastNet.tx) / dt)
      }
    }
    lastNet = { time: now, rx: currentRx, tx: currentTx }
  }
}, { deep: true })

onMounted(() => {
  store.connect()
})

function formatBytes(bytes) {
  if (!bytes || bytes === 0 || isNaN(bytes)) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  if (i < 0) return '0 B';
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}
</script>
