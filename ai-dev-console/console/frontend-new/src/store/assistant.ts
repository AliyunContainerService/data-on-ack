import { create } from 'zustand';
import { getAgentStatus } from '../api/agent';

export interface DiagnoseTarget {
  namespace: string;
  name: string;
  kind: string;
  /** Monotonic id so the drawer consumes each request exactly once. */
  seq: number;
}

interface AssistantState {
  /** Whether the backend agent is enabled (GET /agent/status succeeded with enabled=true). */
  available: boolean;
  checked: boolean;
  open: boolean;
  diagnoseRequest: DiagnoseTarget | null;
  fetchAvailability: () => Promise<void>;
  setOpen: (open: boolean) => void;
  requestDiagnose: (target: { namespace: string; name: string; kind: string }) => void;
  clearDiagnoseRequest: () => void;
}

let diagnoseSeq = 0;

export const useAssistantStore = create<AssistantState>((set, get) => ({
  available: false,
  checked: false,
  open: false,
  diagnoseRequest: null,

  fetchAvailability: async () => {
    if (get().checked) return;
    try {
      const status = await getAgentStatus();
      set({ available: status?.enabled === true, checked: true });
    } catch {
      // 404 (route not registered), network failure or disabled agent:
      // the Assistant entry point stays hidden (edge/offline degradation).
      set({ available: false, checked: true });
    }
  },

  setOpen: (open) => set({ open }),

  requestDiagnose: (target) => {
    diagnoseSeq += 1;
    set({ open: true, diagnoseRequest: { ...target, seq: diagnoseSeq } });
  },

  clearDiagnoseRequest: () => set({ diagnoseRequest: null }),
}));
