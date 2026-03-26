<template>
  <div class="glass-panel p-5 rounded-2xl flex flex-col gap-2 relative overflow-hidden group border border-gray-200/50 dark:border-white/10 shadow-lg">
    <div class="flex justify-between items-start relative z-10">
      <h3 class="text-sm font-semibold text-gray-600 dark:text-gray-400 uppercase tracking-wider">{{ title }}</h3>
      <slot name="icon"></slot>
    </div>
    
    <div class="flex items-baseline gap-2 mt-1 relative z-10">
      <span class="text-4xl font-extrabold tracking-tight bg-clip-text text-transparent bg-gradient-to-br from-gray-900 to-gray-500 dark:from-white dark:to-gray-400">
        {{ value }}
      </span>
      <span v-if="unit" class="text-sm font-semibold text-gray-500 dark:text-gray-400 ml-1">{{ unit }}</span>
    </div>
    
    <div v-if="subtitle" class="text-xs font-medium text-gray-500/80 dark:text-gray-500 mt-2 relative z-10 flex items-center gap-1">
      <slot name="subtitle-icon"></slot>
      {{ subtitle }}
    </div>
    
    <!-- Decorative background glow -->
    <div class="absolute -bottom-16 -right-16 w-48 h-48 bg-blue-500/5 dark:bg-blue-500/10 rounded-full blur-3xl group-hover:bg-blue-500/10 transition-colors duration-700 pointer-events-none"></div>
  </div>
</template>

<script setup>
defineProps({
  title: String,
  value: [String, Number],
  unit: String,
  subtitle: String
})
</script>
