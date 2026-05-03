import type { Variants } from 'motion/react';

export const EASING = {
    easeOutCubic: [0.25, 0.46, 0.45, 0.94] as const,
    easeOutExpo: [0.16, 1, 0.3, 1] as const,
    easeInOutCubic: [0.65, 0, 0.35, 1] as const,
    easeOutQuart: [0.25, 1, 0.5, 1] as const,
    natureRipple: [0.25, 0.46, 0.45, 0.94] as const,
    waterhouseDrift: [0.25, 0.46, 0.45, 0.94] as const,
    waterhouseFloat: [0.16, 1, 0.3, 1] as const,
    waterhouseSurface: [0.16, 1, 0.3, 1] as const,
} as const;

export const SPRING = {
    smooth: {
        type: "spring" as const,
        stiffness: 80,
        damping: 20,
        mass: 1.2,
    },
    gentle: {
        type: "spring" as const,
        stiffness: 70,
        damping: 18,
        mass: 1.5,
    },
    bouncy: {
        type: "spring" as const,
        stiffness: 100,
        damping: 15,
        mass: 1,
    },
    waterhouseHover: {
        type: "spring" as const,
        stiffness: 80,
        damping: 20,
        mass: 1.2,
    },
    waterhouseMagnetic: {
        type: "spring" as const,
        stiffness: 80,
        damping: 20,
        mass: 1.2,
    },
} as const;

export const ENTRANCE_VARIANTS = {
    navbar: {
        initial: {
            opacity: 0,
            scale: 0.95,
        },
        animate: {
            opacity: 1,
            scale: 1,
            transition: SPRING.gentle,
        },
    } as Variants,

    content: {
        initial: {
            opacity: 0,
        },
        animate: {
            opacity: 1,
            transition: {
                duration: 0.25,
                ease: EASING.easeOutCubic,
                delay: 0.04,
            },
        },
    } as Variants,
};
