"use client";

import * as React from "react";
import * as THREE from "three";

/** Decorative scroll-driven 3D backdrop. Pure decoration (aria-hidden); static when reduced motion is requested. */
export function ThreeScene() {
  const ref = React.useRef<HTMLCanvasElement>(null);

  React.useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    let renderer: THREE.WebGLRenderer;
    try {
      renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true });
    } catch {
      return; // WebGL unavailable: the page works without the backdrop
    }
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
    camera.position.set(0, 0, 14);
    scene.add(new THREE.AmbientLight(0xffffff, 1.1));
    const key = new THREE.DirectionalLight(0xffffff, 1.6);
    key.position.set(5, 8, 6);
    scene.add(key);

    const colors = [0xf4573b, 0xfad47f, 0x7b5cf0, 0xd0e8ba, 0xc4def8, 0xf08a6c];
    const geos = [new THREE.IcosahedronGeometry(0.9, 0), new THREE.TorusGeometry(0.7, 0.28, 16, 32), new THREE.BoxGeometry(1.1, 1.1, 1.1), new THREE.SphereGeometry(0.75, 24, 24)];
    const group = new THREE.Group();
    const meshes: { m: THREE.Mesh; speed: number; phase: number }[] = [];
    for (let i = 0; i < 26; i++) {
      const mat = new THREE.MeshStandardMaterial({ color: colors[i % colors.length], roughness: 0.45, metalness: 0.05 });
      const m = new THREE.Mesh(geos[i % geos.length], mat);
      const side = i % 2 === 0 ? -1 : 1;
      m.position.set(side * (4 + Math.random() * 6), (Math.random() - 0.5) * 60 - 10, (Math.random() - 0.5) * 8 - 2);
      m.scale.setScalar(0.6 + Math.random() * 0.9);
      m.rotation.set(Math.random() * 3, Math.random() * 3, 0);
      group.add(m);
      meshes.push({ m, speed: 0.2 + Math.random() * 0.5, phase: Math.random() * 6 });
    }
    scene.add(group);

    const resize = () => {
      const w = window.innerWidth, h = window.innerHeight;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
    };
    resize();
    window.addEventListener("resize", resize);

    let raf = 0;
    const draw = (t: number) => {
      const max = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
      const p = window.scrollY / max;
      camera.position.y = -p * 40;
      group.rotation.y = p * 1.2;
      for (const o of meshes) {
        o.m.rotation.x += reduce ? 0 : 0.004 * o.speed;
        o.m.rotation.y += reduce ? 0 : 0.006 * o.speed;
        o.m.position.x += reduce ? 0 : Math.sin(t / 1500 + o.phase) * 0.003;
      }
      renderer.render(scene, camera);
      if (!reduce) raf = requestAnimationFrame(draw);
    };
    raf = requestAnimationFrame(draw);
    const onScroll = () => { if (reduce) draw(0); };
    window.addEventListener("scroll", onScroll, { passive: true });

    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener("resize", resize);
      window.removeEventListener("scroll", onScroll);
      geos.forEach((g) => g.dispose());
      meshes.forEach((o) => (o.m.material as THREE.Material).dispose());
      renderer.dispose();
    };
  }, []);

  return <canvas ref={ref} aria-hidden className="pointer-events-none fixed inset-0 z-0 h-full w-full opacity-60" />;
}
