import * as React from "react";
import { motion } from "framer-motion";

export const SettingsGroup = React.memo(function SettingsGroup({
  className = "",
  children,
  ...props
}: React.ComponentProps<typeof motion.div>) {
  return (
    <motion.div layout className={`settings-group ${className}`} {...props}>
      {children}
    </motion.div>
  );
});
