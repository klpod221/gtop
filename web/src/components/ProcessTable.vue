<template>
  <div class="glass-panel rounded-2xl overflow-hidden shadow-lg border border-gray-200/50 dark:border-white/10 flex flex-col h-full max-h-[600px]">
    <div class="px-6 py-4 border-b border-gray-100 dark:border-gray-800/50 flex justify-between items-center bg-white/40 dark:bg-gray-900/40 backdrop-blur-sm shrink-0">
      <h3 class="text-sm font-bold text-gray-700 dark:text-gray-300 uppercase tracking-wider">Top Processes</h3>
    </div>
    <div class="overflow-auto flex-1">
      <table class="w-full text-left text-sm text-gray-600 dark:text-gray-400 border-collapse">
        <thead class="sticky top-0 z-10 text-xs uppercase bg-gray-100/90 dark:bg-gray-800/90 backdrop-blur-md text-gray-500 dark:text-gray-500 font-semibold tracking-wider shadow-sm">
          <tr>
            <th class="px-6 py-3">PID</th>
            <th class="px-6 py-3">Name</th>
            <th class="px-6 py-3 text-right">CPU %</th>
            <th class="px-6 py-3 text-right">MEM</th>
            <th class="px-6 py-3 text-right">USER</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-gray-800/50 bg-white/20 dark:bg-transparent">
          <tr v-for="proc in processes" :key="proc.pid" class="hover:bg-white/60 dark:hover:bg-gray-800/40 transition-colors duration-150">
            <td class="px-6 py-3 font-mono text-xs text-gray-400 dark:text-gray-500">{{ proc.pid }}</td>
            <td class="px-6 py-3 font-medium text-gray-800 dark:text-gray-200 truncate max-w-[200px]" :title="proc.cmdline">
              {{ proc.name }}
            </td>
            <td class="px-6 py-3 text-right">
              <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-bold" 
                    :class="proc.cpu_percent > 20 ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400' : 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'">
                {{ proc.cpu_percent.toFixed(1) }}%
              </span>
            </td>
            <td class="px-6 py-3 text-right font-medium">
              {{ formatBytes(proc.mem_rss_bytes) }}
            </td>
            <td class="px-6 py-3 text-right text-xs opacity-70">{{ proc.user }}</td>
          </tr>
          <tr v-if="!processes || processes.length === 0">
            <td colspan="5" class="px-6 py-12 text-center text-gray-500">Waiting for process data...</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
defineProps({
  processes: {
    type: Array,
    default: () => []
  }
})

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}
</script>
