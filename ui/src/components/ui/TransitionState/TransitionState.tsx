import * as React from "react";
import { AnimatePresence, motion } from "framer-motion";

export interface TransitionStateProps {
  isLoading: boolean;
  fallback: React.ReactNode;
  children: React.ReactNode;
}

export function TransitionState({ isLoading, fallback, children }: TransitionStateProps) {
  return (
    <AnimatePresence mode="wait">
      {isLoading ? (
        <motion.div
          key="fallback"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {fallback}
        </motion.div>
      ) : (
        <motion.div
          key="content"
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={{ duration: 0.25, type: "spring", bounce: 0, stiffness: 200 }}
        >
          {children}
        </motion.div>
      )}
    </AnimatePresence>
  );
}
