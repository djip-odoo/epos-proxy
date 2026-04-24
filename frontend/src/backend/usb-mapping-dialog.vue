<script setup>
import { ref, onMounted, onUnmounted } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  GetUSBMappings,
  SaveUSBMapping,
  DeleteUSBMapping,
} from "../../wailsjs/go/main/App";

const open = ref(false);
const mappings = ref({});
const vidpid = ref("");
const type = ref("THERMAL");

onMounted(() => {
  EventsOn("open-usb-mapping-manager", async () => {
    mappings.value = await GetUSBMappings();
    open.value = true;
  });

  window.addEventListener("keydown", handleEsc);
});

onUnmounted(() => {
  window.removeEventListener("keydown", handleEsc);
});

const handleEsc = (e) => {
  if (e.key === "Escape") open.value = false;
};

const addMapping = async () => {
  if (!vidpid.value) return;

  const key = vidpid.value.toLowerCase().trim();

  await SaveUSBMapping(key, type.value);
  mappings.value[key] = type.value;

  vidpid.value = "";
};

const removeMapping = async (key) => {
  await DeleteUSBMapping(key);
  delete mappings.value[key];
};
</script>

<template>
  <teleport to="body">
    <transition enter-active-class="transition duration-200 ease-out" enter-from-class="opacity-0"
      enter-to-class="opacity-100" leave-active-class="transition duration-150 ease-in" leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center p-4">

        <div class="absolute inset-0 bg-black/50" @click="open = false" />

        <!-- Modal -->
        <div class="relative w-full max-w-md rounded-2xl bg-[#f6f6f6] shadow-xl p-6">

          <!-- Header -->
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-lg font-medium text-gray-800">
              USB Printer Mapping
            </h2>
            <button class="w-8 h-8 rounded-full bg-gray-200 text-gray-600 hover:bg-gray-300" @click="open = false">
              ✕
            </button>
          </div>
          <!-- List -->
          <div class="mb-5 max-h-52 overflow-y-auto space-y-2">
            <div v-for="(val, key) in mappings" :key="key" class="flex justify-between items-center 
                    bg-white border border-gray-200 
                    rounded-xl px-4 py-3 
                    shadow-sm hover:shadow-md 
                    transition-all duration-200">
              <div class="flex flex-col">
                <span class="text-sm font-semibold text-gray-900 tracking-wide">
                  {{ key }}
                </span>

                <span class="mt-1 inline-block text-xs font-medium 
                        px-2 py-0.5 rounded-full 
                        bg-blue-50 text-blue-600 w-fit">
                  {{ val }}
                </span>
              </div>

              <button class="flex items-center gap-1 cursor-pointer
                      text-red-500 text-xs font-medium 
                      px-2 py-1 rounded-md
                      hover:bg-red-50 hover:text-red-600
                      transition-all duration-150" @click="removeMapping(key)">
                🗑 Delete
              </button>
            </div>

            <div v-if="!Object.keys(mappings).length" class="text-center text-gray-400 py-4">
              No mappings yet
            </div>
          </div>

          <!-- Input -->
          <input v-model="vidpid" placeholder="VID:PID (e.g. 04b8:0e32)"
            class="w-full px-4 py-2 rounded-lg border border-gray-300 bg-white focus:outline-none focus:border-purple-400 mb-4" />

          <!-- Type -->
          <div class="mb-4">
            <p class="text-sm text-gray-600 mb-2">Printer Type</p>

            <div class="flex gap-4">
              <label class="flex items-center gap-2 text-sm text-gray-700">
                <input type="radio" value="THERMAL" v-model="type" />
                Receipt / Label
              </label>

              <label class="flex items-center gap-2 text-sm text-gray-700">
                <input type="radio" value="OFFICE" v-model="type" />
                Office
              </label>
            </div>
          </div>

          <button class="w-full py-2 rounded-lg bg-[#7c5a72] text-white hover:bg-[#6b4c62] transition"
            @click="addMapping">
            Add
          </button>
        </div>
      </div>
    </transition>
  </teleport>
</template>
